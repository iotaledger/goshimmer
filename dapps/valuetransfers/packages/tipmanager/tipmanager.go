package tipmanager

import (
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
	"github.com/iotaledger/hive.go/events"
)

// TipManager manages liked tips, tips of all branches and emits events for their removal and addition.
type TipManager struct {
	// tips are all currently liked tips.
	tips *datastructure.RandomMap
	// branches contains data for every branch and its current tips.
	branches map[branchmanager.BranchID]*Branch
	mutex    sync.Mutex // TODO: should be locked at branch level
	Events   Events
}

// New creates a new TipManager.
func New() *TipManager {
	return &TipManager{
		tips:     datastructure.NewRandomMap(),
		branches: make(map[branchmanager.BranchID]*Branch),
		Events: Events{
			TipAdded:   events.NewEvent(payloadIDEvent),
			TipRemoved: events.NewEvent(payloadIDEvent),
		},
	}
}

// Tip represents a tip with its corresponding branchID and a reverse mapping to branches that reference it.
type Tip struct {
	id       payload.ID
	branchID branchmanager.BranchID
	// referencedByOtherBranches is a reverse mapping to all branches that reference this tip as entryPoint.
	referencedByOtherBranches map[branchmanager.BranchID]*Branch
}

func newTip(id payload.ID, branchID branchmanager.BranchID) *Tip {
	return &Tip{
		id:                        id,
		branchID:                  branchID,
		referencedByOtherBranches: make(map[branchmanager.BranchID]*Branch),
	}
}

// Branch represents a branch with its tips and entryPoints.
type Branch struct {
	branchID branchmanager.BranchID
	liked    bool

	// tips are the current tips in this branch.
	tips map[payload.ID]*Tip
	// entryPoints are tips in other branches that are referenced by this branch.
	entryPoints map[payload.ID]*Tip
}

func newBranch(branchID branchmanager.BranchID, liked bool) *Branch {
	return &Branch{
		branchID:    branchID,
		tips:        make(map[payload.ID]*Tip),
		entryPoints: make(map[payload.ID]*Tip),
		liked:       liked,
	}
}

// LikableBrancher defines an interface for a likable branch.
type LikableBrancher interface {
	ID() branchmanager.BranchID
	Liked() bool
}

// AddTip adds the given value object as a tip in the given branch.
// If the branch is liked it is also added to t.tips.
// Parents are handled depending on the relation (same or different branch).
func (t *TipManager) AddTip(valueObject *payload.Payload, b LikableBrancher) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	objectID := valueObject.ID()
	parent1ID := valueObject.TrunkID()
	parent2ID := valueObject.BranchID()

	branch, ok := t.branches[b.ID()]
	if !ok {
		branch = newBranch(b.ID(), b.Liked())
		t.branches[b.ID()] = branch
	}

	// add new tip to branch.tips
	tip := newTip(objectID, branch.branchID)
	branch.tips[objectID] = tip

	// TODO: retrieve parents' branches
	parent1branch := branch
	addTipHandleParent(parent1ID, parent1branch, branch)
	parent2branch := branch
	addTipHandleParent(parent2ID, parent2branch, branch)

	// add to t.tips and remove parents from t.tips if branch is liked
	if branch.liked {
		t.tips.Set(tip.id, tip)
		t.tips.Delete(parent1ID)
		t.tips.Delete(parent2ID)
	}
}

// Tips returns two randomly selected tips.
func (t *TipManager) Tips() (parent1ObjectID, parent2ObjectID payload.ID) {
	// TODO: this might be over locking
	t.mutex.Lock()
	defer t.mutex.Unlock()

	tip := t.tips.RandomEntry()
	if tip == nil {
		parent1ObjectID = payload.GenesisID
		parent2ObjectID = payload.GenesisID
		return
	}

	parent1ObjectID = tip.(*Tip).id

	if t.tips.Size() == 1 {
		parent2ObjectID = parent1ObjectID
		return
	}

	parent2ObjectID = t.tips.RandomEntry().(*Tip).id
	// select 2 distinct tips if there's more than 1 tip available
	for parent1ObjectID == parent2ObjectID && t.tips.Size() > 1 {
		parent2ObjectID = t.tips.RandomEntry().(*Tip).id
	}

	return
}

// TipCount returns the total liked tips.
func (t *TipManager) TipCount() int {
	// TODO: this might be over locking
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.tips.Size()
}

// OnBranchLiked is called when a branch is liked.
// It adds the branch's tips to t.tips and removes tips from referenced branches from t.tips.
func (t *TipManager) OnBranchLiked(branchID branchmanager.BranchID) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	branch := t.branches[branchID]
	branch.liked = true

	// add tips of current branch
	for _, tip := range branch.tips {
		addTipIfNotReferencedByLikedBranch(tip, t)
	}

	// remove tips from referenced branches
	for objectID := range branch.entryPoints {
		t.tips.Delete(objectID)
	}
}

// OnBranchDisliked is called when a branch is disliked.
// It removes the branch's tips from t.tips and adds tips from referenced branches back to t.tips.
func (t *TipManager) OnBranchDisliked(branchID branchmanager.BranchID) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	branch := t.branches[branchID]
	branch.liked = false

	// remove tips of current branch
	for _, tip := range branch.tips {
		t.tips.Delete(tip.id)
	}

	// add tips from referenced branches
	for _, tipFromOtherBranch := range branch.entryPoints {
		addTipIfNotReferencedByLikedBranch(tipFromOtherBranch, t)
	}
}

// OnBranchMerged is called when a branch is merged.
// TODO: it should perform some cleanup logic
func (t *TipManager) OnBranchMerged(branchID branchmanager.BranchID) {
	// remove all tips from t.tips
	// remove tips out of entryPoints of other branches
}

// removeTipAsEntryPoint removes the tip from all its referenced branches' entry point map.
func removeTipAsEntryPoint(tip *Tip) {
	for _, branch := range tip.referencedByOtherBranches {
		delete(branch.entryPoints, tip.id)
	}
}

// addTipHandleParent handles a tip's parent when adding it.
// If the parent is in the same branch it is removed as a tip.
// If the parent is not in the same branch a two-way mapping from parent to branch and branch to parent is created.
func addTipHandleParent(parentID payload.ID, parentBranch *Branch, branch *Branch) {
	parentTip, ok := parentBranch.tips[parentID]
	// parent is not a tip anymore
	if !ok {
		return
	}

	if parentBranch.branchID == branch.branchID {
		// remove parents out of branch.tips
		delete(branch.tips, parentID)
		// properly remove parent -> not a tip anymore!
		removeTipAsEntryPoint(parentTip)
	} else {
		// add reference to current branch to parent tip
		parentTip.referencedByOtherBranches[branch.branchID] = branch

		// add reference to parent to current branch
		branch.entryPoints[parentID] = parentTip
	}
}

// addTipIfNotReferencedByLikedBranch adds a tip to t.tips
// only if it is not referenced by any liked branch.
func addTipIfNotReferencedByLikedBranch(tip *Tip, t *TipManager) {
	addTip := true

	// if there's any liked branch referencing this tip we do not add it to t.tips
	for _, b := range tip.referencedByOtherBranches {
		if b.liked {
			addTip = false
			break
		}
	}

	if addTip {
		t.tips.Set(tip.id, tip)
	}
}
