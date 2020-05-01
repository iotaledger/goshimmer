package branchmanager

import (
	"container/list"
	"fmt"
	"sort"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

// BranchManager is an entity, that manages the branches of a UTXODAG. It offers methods to add, delete and modify
// Branches. It automatically keeps track of maintaining a valid perception.
type BranchManager struct {
	branchStorage         *objectstorage.ObjectStorage
	childBranchStorage    *objectstorage.ObjectStorage
	conflictStorage       *objectstorage.ObjectStorage
	conflictMemberStorage *objectstorage.ObjectStorage

	Events *Events

	onAddBranchToConflictClosure *events.Closure
}

// New is the constructor of the BranchManager. It creates a new instance with the given storage details.
func New(badgerInstance *badger.DB) (result *BranchManager) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &BranchManager{
		branchStorage:         osFactory.New(osBranch, osBranchFactory, osBranchOptions...),
		childBranchStorage:    osFactory.New(osChildBranch, osChildBranchFactory, osChildBranchOptions...),
		conflictStorage:       osFactory.New(osConflict, osConflictFactory, osConflictOptions...),
		conflictMemberStorage: osFactory.New(osConflictMember, osConflictMemberFactory, osConflictMemberOptions...),
	}
	result.init()

	return
}

// Branch loads a Branch from the objectstorage.
func (branchManager *BranchManager) Branch(branchID BranchID) *CachedBranch {
	return &CachedBranch{CachedObject: branchManager.branchStorage.Load(branchID.Bytes())}
}

// ChildBranches loads the ChildBranches that are forking off, of the given Branch.
func (branchManager *BranchManager) ChildBranches(branchID BranchID) CachedChildBranches {
	childBranches := make(CachedChildBranches, 0)
	branchManager.childBranchStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		childBranches = append(childBranches, &CachedChildBranch{CachedObject: cachedObject})

		return true
	}, branchID.Bytes())

	return childBranches
}

// Conflict loads the corresponding Conflict from the objectstorage.
func (branchManager *BranchManager) Conflict(conflictID ConflictID) *CachedConflict {
	return &CachedConflict{CachedObject: branchManager.conflictStorage.Load(conflictID.Bytes())}
}

// ConflictMembers loads the ConflictMembers that are part of the same Conflict from the objectstorage.
func (branchManager *BranchManager) ConflictMembers(conflictID ConflictID) CachedConflictMembers {
	conflictMembers := make(CachedConflictMembers, 0)
	branchManager.conflictMemberStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		conflictMembers = append(conflictMembers, &CachedConflictMember{CachedObject: cachedObject})

		return true
	}, conflictID.Bytes())

	return conflictMembers
}

// AddBranch adds a new Branch to the branch-DAG and connects the branch with its parents, children and conflicts. It
// automatically creates the Conflicts if they don't exist.
func (branchManager *BranchManager) AddBranch(branchID BranchID, parentBranches []BranchID, conflicts []ConflictID) (cachedBranch *CachedBranch, newBranchAdded bool) {
	// create the branch
	cachedBranch = &CachedBranch{CachedObject: branchManager.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) objectstorage.StorableObject {
		newBranch := NewBranch(branchID, parentBranches, conflicts)
		newBranch.Persist()
		newBranch.SetModified()

		newBranchAdded = true

		return newBranch
	})}

	if !newBranchAdded {
		return
	}

	// create the referenced entities and references
	cachedBranch.Retain().Consume(func(branch *Branch) {
		// store parent references
		for _, parentBranchID := range parentBranches {
			if cachedChildBranch, stored := branchManager.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID)); stored {
				cachedChildBranch.Release()
			}
		}

		// store conflict + conflict references
		for _, conflictID := range conflicts {
			(&CachedConflict{CachedObject: branchManager.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) objectstorage.StorableObject {
				newConflict := NewConflict(conflictID)
				newConflict.Persist()
				newConflict.SetModified()

				return newConflict
			})}).Consume(func(conflict *Conflict) {
				if cachedConflictMember, stored := branchManager.conflictMemberStorage.StoreIfAbsent(NewConflictMember(conflictID, branchID)); stored {
					conflict.IncreaseMemberCount()

					cachedConflictMember.Release()
				}
			})
		}
	})

	return
}

// AddBranchToConflict adds the branch to a conflict. Since there are certain constraints (i.e. the conflict being added
// as additional ConflictMember references for reverse lookups), we expose this method here as part of the BranchManager
// instead of exposing the addConflict method in the Branch itself.
func (branchManager *BranchManager) AddBranchToConflict(branchID BranchID, conflictID ConflictID) {
	branchManager.Branch(branchID).Consume(func(branch *Branch) {
		(&CachedConflict{CachedObject: branchManager.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) objectstorage.StorableObject {
			newConflict := NewConflict(conflictID)
			newConflict.Persist()
			newConflict.SetModified()

			return newConflict
		})}).Consume(func(conflict *Conflict) {
			if branch.addConflict(conflictID) {
				if cachedConflictMember, stored := branchManager.conflictMemberStorage.StoreIfAbsent(NewConflictMember(conflictID, branchID)); stored {
					conflict.IncreaseMemberCount()

					cachedConflictMember.Release()
				}
			}
		})
	})
}

// BranchesConflicting returns true if the given Branches are part of the same Conflicts and can therefore not be
// merged.
func (branchManager *BranchManager) BranchesConflicting(branchIds ...BranchID) (branchesConflicting bool, err error) {
	// iterate through parameters and collect conflicting branches
	conflictingBranches := make(map[BranchID]types.Empty)
	processedBranches := make(map[BranchID]types.Empty)
	for _, branchID := range branchIds {
		// add the current branch to the stack of branches to check
		ancestorStack := list.New()
		ancestorStack.PushBack(branchID)

		// iterate over all ancestors and collect the conflicting branches
		for ancestorStack.Len() >= 1 {
			// retrieve branch from stack
			firstElement := ancestorStack.Front()
			ancestorBranchID := firstElement.Value.(BranchID)
			ancestorStack.Remove(firstElement)

			// abort if we have seen this branch already
			if _, processedAlready := processedBranches[ancestorBranchID]; processedAlready {
				continue
			}

			// unpack the branch and abort if we failed to load it
			cachedBranch := branchManager.Branch(branchID)
			branch := cachedBranch.Unwrap()
			if branch == nil {
				err = fmt.Errorf("failed to load branch '%s'", ancestorBranchID)

				cachedBranch.Release()

				return
			}

			// add the parents of the current branch to the list if branches to check
			for _, parentBranchID := range branch.ParentBranches() {
				ancestorStack.PushBack(parentBranchID)
			}

			// abort the following checks if the branch is aggregated
			if branch.IsAggregated() {
				cachedBranch.Release()

				continue
			}

			// determine conflicts of this branch
			conflicts := branch.Conflicts()
			if len(conflicts) == 0 {
				ancestorStack.PushBack(ancestorBranchID)

				cachedBranch.Release()

				continue
			}

			// iterate through the conflicts and take note of the conflicting branches
			for conflictID := range conflicts {
				for _, cachedConflictMember := range branchManager.ConflictMembers(conflictID) {
					// unwrap the current ConflictMember
					conflictMember := cachedConflictMember.Unwrap()
					if conflictMember == nil {
						cachedConflictMember.Release()

						continue
					}

					// abort if this branch was found as a conflict of another branch already
					if _, branchesConflicting = conflictingBranches[conflictMember.BranchID()]; branchesConflicting {
						cachedConflictMember.Release()
						cachedBranch.Release()

						return
					}

					// store the current conflict in the list of seen conflicting branches
					conflictingBranches[conflictMember.BranchID()] = types.Void

					cachedConflictMember.Release()
				}
			}

			cachedBranch.Release()
		}
	}

	return
}

// InheritBranches takes a list of BranchIDs and tries to "inherit" the given IDs into a new Branch.
func (branchManager *BranchManager) InheritBranches(branches ...BranchID) (cachedAggregatedBranch *CachedBranch, err error) {
	// return the MasterBranch if we have no branches in the parameters
	if len(branches) == 0 {
		cachedAggregatedBranch = branchManager.Branch(MasterBranchID)

		return
	}

	if len(branches) == 1 {
		cachedAggregatedBranch = branchManager.Branch(branches[0])

		return
	}

	// check if the branches are conflicting
	branchesConflicting, err := branchManager.BranchesConflicting(branches...)
	if err != nil {
		return
	}
	if branchesConflicting {
		err = fmt.Errorf("the branches are conflicting")

		return
	}

	// filter out duplicates and shared ancestor Branches (abort if we faced an error)
	deepestCommonAncestors, err := branchManager.findDeepestCommonAncestorBranches(branches...)
	if err != nil {
		return
	}

	// if there is only one branch that we found, then we are done
	if len(deepestCommonAncestors) == 1 {
		for _, firstBranchInList := range deepestCommonAncestors {
			cachedAggregatedBranch = firstBranchInList
		}

		return
	}

	// if there is more than one parents: aggregate
	aggregatedBranchID, aggregatedBranchParents, err := branchManager.determineAggregatedBranchDetails(deepestCommonAncestors)
	if err != nil {
		return
	}

	// store the aggregated branch
	cachedAggregatedBranch, _ = branchManager.AddBranch(aggregatedBranchID, aggregatedBranchParents, []ConflictID{})

	return
}

// SetBranchPreferred is the method that allows us to modify the preferred flag of a branch.
func (branchManager *BranchManager) SetBranchPreferred(branchID BranchID, preferred bool) (modified bool, err error) {
	return branchManager.setBranchPreferred(branchManager.Branch(branchID), preferred)
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (branchManager *BranchManager) Prune() (err error) {
	for _, storage := range []*objectstorage.ObjectStorage{
		branchManager.branchStorage,
	} {
		if err = storage.Prune(); err != nil {
			return
		}
	}

	branchManager.init()

	return
}

func (branchManager *BranchManager) init() {
	cachedBranch, branchAdded := branchManager.AddBranch(MasterBranchID, []BranchID{}, []ConflictID{})
	if !branchAdded {
		cachedBranch.Release()

		return
	}

	cachedBranch.Consume(func(branch *Branch) {
		branch.SetPreferred(true)
		branch.SetLiked(true)
	})
}

func (branchManager *BranchManager) setBranchPreferred(cachedBranch *CachedBranch, preferred bool) (modified bool, err error) {
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = fmt.Errorf("failed to unwrap branch")

		return
	}

	if !preferred {
		if modified = branch.SetPreferred(false); modified {
			branchManager.Events.BranchUnpreferred.Trigger(cachedBranch)

			branchManager.propagateDislike(cachedBranch.Retain())
		}

		return
	}

	for conflictID := range branch.Conflicts() {
		branchManager.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			if conflictMember.BranchID() == branch.ID() {
				return
			}

			_, _ = branchManager.setBranchPreferred(branchManager.Branch(conflictMember.BranchID()), false)
		})
	}

	if modified = branch.SetPreferred(true); !modified {
		return
	}

	branchManager.Events.BranchPreferred.Trigger(cachedBranch)

	err = branchManager.propagateLike(cachedBranch.Retain())

	return
}

func (branchManager *BranchManager) propagateLike(cachedBranch *CachedBranch) (err error) {
	// unpack CachedBranch and abort of the branch doesn't exist or isn't preferred
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil || !branch.Preferred() {
		return
	}

	// check if parents are liked
	for _, parentBranchID := range branch.ParentBranches() {
		// abort, if the parent branch can not be loaded
		cachedParentBranch := branchManager.Branch(parentBranchID)
		parentBranch := cachedParentBranch.Unwrap()
		if parentBranch == nil {
			cachedParentBranch.Release()

			return fmt.Errorf("failed to load parent branch '%s' of branch '%s'", parentBranchID, branch.ID())
		}

		// abort if the parent branch is not liked
		if !parentBranch.Liked() {
			cachedParentBranch.Release()

			return
		}

		cachedParentBranch.Release()
	}

	// abort if the branch was liked already
	if !branch.SetLiked(true) {
		return
	}

	// trigger events
	branchManager.Events.BranchLiked.Trigger(cachedBranch)

	// propagate liked checks to the children
	for _, cachedChildBranch := range branchManager.ChildBranches(branch.ID()) {
		childBranch := cachedChildBranch.Unwrap()
		if childBranch == nil {
			cachedChildBranch.Release()

			continue
		}

		if err = branchManager.propagateLike(branchManager.Branch(childBranch.ChildID())); err != nil {
			cachedChildBranch.Release()

			return
		}

		cachedChildBranch.Release()
	}

	return
}

func (branchManager *BranchManager) propagateDislike(cachedBranch *CachedBranch) {
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil || !branch.SetLiked(false) {
		return
	}

	branchManager.Events.BranchDisliked.Trigger(cachedBranch)

	branchManager.ChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
		branchManager.propagateDislike(branchManager.Branch(childBranch.ChildID()))
	})
}

func (branchManager *BranchManager) determineAggregatedBranchDetails(deepestCommonAncestors CachedBranches) (aggregatedBranchID BranchID, aggregatedBranchParents []BranchID, err error) {
	aggregatedBranchParents = make([]BranchID, len(deepestCommonAncestors))

	i := 0
	aggregatedBranchConflictParents := make(CachedBranches)
	for branchID, cachedBranch := range deepestCommonAncestors {
		// release all following entries if we have encountered an error
		if err != nil {
			cachedBranch.Release()

			continue
		}

		// store BranchID as parent
		aggregatedBranchParents[i] = branchID
		i++

		// abort if we could not unwrap the Branch (should never happen)
		branch := cachedBranch.Unwrap()
		if branch == nil {
			cachedBranch.Release()

			err = fmt.Errorf("failed to unwrap brach '%s'", branchID)

			continue
		}

		if branch.IsAggregated() {
			aggregatedBranchConflictParents[branchID] = cachedBranch

			continue
		}

		err = branchManager.collectClosestConflictAncestors(branch, aggregatedBranchConflictParents)

		cachedBranch.Release()
	}

	if err != nil {
		aggregatedBranchConflictParents.Release()
		aggregatedBranchConflictParents = nil

		return
	}

	aggregatedBranchID = branchManager.generateAggregatedBranchID(aggregatedBranchConflictParents)

	return
}

func (branchManager *BranchManager) generateAggregatedBranchID(aggregatedBranches CachedBranches) BranchID {
	counter := 0
	branchIDs := make([]BranchID, len(aggregatedBranches))
	for branchID, cachedBranch := range aggregatedBranches {
		branchIDs[counter] = branchID

		counter++

		cachedBranch.Release()
	}

	sort.Slice(branchIDs, func(i, j int) bool {
		for k := 0; k < len(branchIDs[k]); k++ {
			if branchIDs[i][k] < branchIDs[j][k] {
				return true
			} else if branchIDs[i][k] > branchIDs[j][k] {
				return false
			}
		}

		return false
	})

	marshalUtil := marshalutil.New(BranchIDLength * len(branchIDs))
	for _, branchID := range branchIDs {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	return blake2b.Sum256(marshalUtil.Bytes())
}

func (branchManager *BranchManager) collectClosestConflictAncestors(branch *Branch, closestConflictAncestors CachedBranches) (err error) {
	// initialize stack
	stack := list.New()
	for _, parentRealityID := range branch.ParentBranches() {
		stack.PushBack(parentRealityID)
	}

	// work through stack
	processedBranches := make(map[BranchID]types.Empty)
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchID := stack.Front().Value.(BranchID)

			// abort if the parent has been processed already
			if _, branchProcessed := processedBranches[parentBranchID]; branchProcessed {
				return nil
			}
			processedBranches[parentBranchID] = types.Void

			// load parent branch from database
			cachedParentBranch := branchManager.Branch(parentBranchID)

			// abort if the parent branch could not be found (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to load branch '%s'", parentBranchID)
			}

			// if the parent Branch is not aggregated, then we have found the closest conflict ancestor
			if !parentBranch.IsAggregated() {
				closestConflictAncestors[parentBranchID] = cachedParentBranch

				return nil
			}

			// queue parents for additional check (recursion)
			for _, parentRealityID := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityID)
			}

			// release the branch (we don't need it anymore)
			cachedParentBranch.Release()

			return nil
		}()

		if err != nil {
			return
		}
	}

	return
}

// findDeepestCommonAncestorBranches takes a number of BranchIds and determines the most specialized Branches (furthest
// away from the MasterBranch) in that list, that contains all of the named BranchIds.
//
// Example: If we hand in "A, B" and B has A as its parent, then the result will contain the Branch B, because B is a
//          child of A.
func (branchManager *BranchManager) findDeepestCommonAncestorBranches(branches ...BranchID) (result CachedBranches, err error) {
	result = make(CachedBranches)

	processedBranches := make(map[BranchID]types.Empty)
	for _, branchID := range branches {
		err = func() error {
			// continue, if we have processed this branch already
			if _, exists := processedBranches[branchID]; exists {
				return nil
			}
			processedBranches[branchID] = types.Void

			// load branch from objectstorage
			cachedBranch := branchManager.Branch(branchID)

			// abort if we could not load the CachedBranch
			branch := cachedBranch.Unwrap()
			if branch == nil {
				cachedBranch.Release()

				return fmt.Errorf("could not load branch '%s'", branchID)
			}

			// check branches position relative to already aggregated branches
			for aggregatedBranchID, cachedAggregatedBranch := range result {
				// abort if we can not load the branch
				aggregatedBranch := cachedAggregatedBranch.Unwrap()
				if aggregatedBranch == nil {
					return fmt.Errorf("could not load branch '%s'", aggregatedBranchID)
				}

				// if the current branch is an ancestor of an already aggregated branch, then we have found the more
				// "specialized" branch already and keep it
				if isAncestor, ancestorErr := branchManager.branchIsAncestorOfBranch(branch, aggregatedBranch); isAncestor || ancestorErr != nil {
					return ancestorErr
				}

				// check if the aggregated Branch is an ancestor of the current Branch and abort if we face an error
				isAncestor, ancestorErr := branchManager.branchIsAncestorOfBranch(aggregatedBranch, branch)
				if ancestorErr != nil {
					return ancestorErr
				}

				// if the aggregated branch is an ancestor of the current branch, then we have found a more specialized
				// Branch and replace the old one with this one.
				if isAncestor {
					// replace aggregated branch if we have found a more specialized on
					delete(result, aggregatedBranchID)
					cachedAggregatedBranch.Release()

					result[branchID] = cachedBranch

					return nil
				}
			}

			// store the branch as a new aggregate candidate if it was not found to be in any relation with the already
			// aggregated ones.
			result[branchID] = cachedBranch

			return nil
		}()

		// abort if an error occurred while processing the current branch
		if err != nil {
			result.Release()
			result = nil

			return
		}
	}

	return
}

func (branchManager *BranchManager) branchIsAncestorOfBranch(ancestor *Branch, descendant *Branch) (isAncestor bool, err error) {
	if ancestor.ID() == descendant.ID() {
		return true, nil
	}

	ancestorBranches, err := branchManager.getAncestorBranches(descendant)
	if err != nil {
		return
	}

	ancestorBranches.Consume(func(ancestorOfDescendant *Branch) {
		if ancestorOfDescendant.ID() == ancestor.ID() {
			isAncestor = true
		}
	})

	return
}

func (branchManager *BranchManager) getAncestorBranches(branch *Branch) (ancestorBranches CachedBranches, err error) {
	// initialize result
	ancestorBranches = make(CachedBranches)

	// initialize stack
	stack := list.New()
	for _, parentRealityID := range branch.ParentBranches() {
		stack.PushBack(parentRealityID)
	}

	// work through stack
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchID := stack.Front().Value.(BranchID)

			// abort if the parent has been processed already
			if _, branchProcessed := ancestorBranches[parentBranchID]; branchProcessed {
				return nil
			}

			// load parent branch from database
			cachedParentBranch := branchManager.Branch(parentBranchID)

			// abort if the parent branch could not be founds (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to unwrap branch '%s'", parentBranchID)
			}

			// store parent branch in result
			ancestorBranches[parentBranchID] = cachedParentBranch

			// queue parents for additional check (recursion)
			for _, parentRealityID := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityID)
			}

			return nil
		}()

		// abort if an error occurs while trying to process the parents
		if err != nil {
			ancestorBranches.Release()
			ancestorBranches = nil

			return
		}
	}

	return
}
