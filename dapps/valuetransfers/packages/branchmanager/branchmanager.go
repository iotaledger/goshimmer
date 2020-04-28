package branchmanager

import (
	"container/list"
	"fmt"
	"sort"

	"github.com/dgraph-io/badger/v2"
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
}

// New is the constructor of the BranchManager. It creates a new instance with the given storage details.
func New(badgerInstance *badger.DB) (result *BranchManager) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &BranchManager{
		branchStorage: osFactory.New(osBranch, osBranchFactory, osBranchOptions...),
	}
	result.init()

	return
}

func (branchManager *BranchManager) init() {
	branchManager.branchStorage.StoreIfAbsent(NewBranch(MasterBranchID, []BranchID{}, []ConflictID{}))
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

// AddBranch adds a new Branch to the branch-DAG.
func (branchManager *BranchManager) AddBranch(branch *Branch) *CachedBranch {
	return &CachedBranch{CachedObject: branchManager.branchStorage.ComputeIfAbsent(branch.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		branch.Persist()
		branch.SetModified()

		return branch
	})}
}

// Branch loads a Branch from the objectstorage.
func (branchManager *BranchManager) Branch(branchID BranchID) *CachedBranch {
	return &CachedBranch{CachedObject: branchManager.branchStorage.Load(branchID.Bytes())}
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

	newAggregatedBranchCreated := false
	cachedAggregatedBranch = &CachedBranch{CachedObject: branchManager.branchStorage.ComputeIfAbsent(aggregatedBranchID.Bytes(), func(key []byte) (object objectstorage.StorableObject) {
		aggregatedReality := NewBranch(aggregatedBranchID, aggregatedBranchParents, []ConflictID{})

		// TODO: FIX
		/*
			for _, parentRealityId := range aggregatedBranchParents {
				tangle.Branch(parentRealityId).Consume(func(branch *Branch) {
					branch.RegisterSubReality(aggregatedRealityId)
				})
			}
		*/

		aggregatedReality.SetModified()

		newAggregatedBranchCreated = true

		return aggregatedReality
	})}

	if !newAggregatedBranchCreated {
		fmt.Println("1")
		// TODO: FIX
		/*
			aggregatedBranch := cachedAggregatedBranch.Unwrap()

			for _, realityId := range aggregatedBranchParents {
				if aggregatedBranch.AddParentReality(realityId) {
					tangle.Branch(realityId).Consume(func(branch *Branch) {
						branch.RegisterSubReality(aggregatedRealityId)
					})
				}
			}
		*/
	}

	return
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

// SetBranchPreferred is the method that allows us to modify the preferred flag of a transaction.
func (branchManager *BranchManager) SetBranchPreferred(branchID BranchID, preferred bool) (modified bool, err error) {
	return branchManager.setBranchPreferred(branchManager.Branch(branchID), preferred)
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
