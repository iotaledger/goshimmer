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

type BranchManager struct {
	branchStorage         *objectstorage.ObjectStorage
	childBranchStorage    *objectstorage.ObjectStorage
	conflictStorage       *objectstorage.ObjectStorage
	conflictMemberStorage *objectstorage.ObjectStorage

	Events *Events
}

func New(badgerInstance *badger.DB) (result *BranchManager) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &BranchManager{
		branchStorage: osFactory.New(osBranch, osBranchFactory, osBranchOptions...),
	}
	result.init()

	return
}

func (branchManager *BranchManager) init() {
	branchManager.branchStorage.StoreIfAbsent(NewBranch(MasterBranchId, []BranchId{}, []ConflictId{}))
}

func (branchManager *BranchManager) Conflict(conflictId ConflictId) *CachedConflict {
	return &CachedConflict{CachedObject: branchManager.conflictStorage.Load(conflictId.Bytes())}
}

func (branchManager *BranchManager) ConflictMembers(conflictId ConflictId) CachedConflictMembers {
	conflictMembers := make(CachedConflictMembers, 0)
	branchManager.conflictMemberStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		conflictMembers = append(conflictMembers, &CachedConflictMember{CachedObject: cachedObject})

		return true
	}, conflictId.Bytes())

	return conflictMembers
}

func (branchManager *BranchManager) AddBranch(branch *Branch) *CachedBranch {
	return &CachedBranch{CachedObject: branchManager.branchStorage.ComputeIfAbsent(branch.Id().Bytes(), func(key []byte) objectstorage.StorableObject {
		branch.Persist()
		branch.SetModified()

		return branch
	})}
}

func (branchManager *BranchManager) GetBranch(branchId BranchId) *CachedBranch {
	return &CachedBranch{CachedObject: branchManager.branchStorage.Load(branchId.Bytes())}
}

func (branchManager *BranchManager) InheritBranches(branches ...BranchId) (cachedAggregatedBranch *CachedBranch, err error) {
	// return the MasterBranch if we have no branches in the parameters
	if len(branches) == 0 {
		cachedAggregatedBranch = branchManager.GetBranch(MasterBranchId)

		return
	}

	if len(branches) == 1 {
		cachedAggregatedBranch = branchManager.GetBranch(branches[0])

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
	aggregatedBranchId, aggregatedBranchParents, err := branchManager.determineAggregatedBranchDetails(deepestCommonAncestors)
	if err != nil {
		return
	}

	newAggregatedBranchCreated := false
	cachedAggregatedBranch = &CachedBranch{CachedObject: branchManager.branchStorage.ComputeIfAbsent(aggregatedBranchId.Bytes(), func(key []byte) (object objectstorage.StorableObject) {
		aggregatedReality := NewBranch(aggregatedBranchId, aggregatedBranchParents, []ConflictId{})

		// TODO: FIX
		/*
			for _, parentRealityId := range aggregatedBranchParents {
				tangle.GetBranch(parentRealityId).Consume(func(branch *Branch) {
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
					tangle.GetBranch(realityId).Consume(func(branch *Branch) {
						branch.RegisterSubReality(aggregatedRealityId)
					})
				}
			}
		*/
	}

	return
}

func (branchManager *BranchManager) ChildBranches(branchId BranchId) CachedChildBranches {
	childBranches := make(CachedChildBranches, 0)
	branchManager.childBranchStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		childBranches = append(childBranches, &CachedChildBranch{CachedObject: cachedObject})

		return true
	}, branchId.Bytes())

	return childBranches
}

func (branchManager *BranchManager) SetBranchPreferred(branchId BranchId, preferred bool) (modified bool, err error) {
	return branchManager.setBranchPreferred(branchManager.GetBranch(branchId), preferred)
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

	for conflictId := range branch.Conflicts() {
		branchManager.ConflictMembers(conflictId).Consume(func(conflictMember *ConflictMember) {
			if conflictMember.BranchId() == branch.Id() {
				return
			}

			_, _ = branchManager.setBranchPreferred(branchManager.GetBranch(conflictMember.BranchId()), false)
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
	for _, parentBranchId := range branch.ParentBranches() {
		// abort, if the parent branch can not be loaded
		cachedParentBranch := branchManager.GetBranch(parentBranchId)
		parentBranch := cachedParentBranch.Unwrap()
		if parentBranch == nil {
			cachedParentBranch.Release()

			return fmt.Errorf("failed to load parent branch '%s' of branch '%s'", parentBranchId, branch.Id())
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
	for _, cachedChildBranch := range branchManager.ChildBranches(branch.Id()) {
		childBranch := cachedChildBranch.Unwrap()
		if childBranch == nil {
			cachedChildBranch.Release()

			continue
		}

		if err = branchManager.propagateLike(branchManager.GetBranch(childBranch.Id())); err != nil {
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

	branchManager.ChildBranches(branch.Id()).Consume(func(childBranch *ChildBranch) {
		branchManager.propagateDislike(branchManager.GetBranch(childBranch.Id()))
	})
}

func (branchManager *BranchManager) determineAggregatedBranchDetails(deepestCommonAncestors CachedBranches) (aggregatedBranchId BranchId, aggregatedBranchParents []BranchId, err error) {
	aggregatedBranchParents = make([]BranchId, len(deepestCommonAncestors))

	i := 0
	aggregatedBranchConflictParents := make(CachedBranches)
	for branchId, cachedBranch := range deepestCommonAncestors {
		// release all following entries if we have encountered an error
		if err != nil {
			cachedBranch.Release()

			continue
		}

		// store BranchId as parent
		aggregatedBranchParents[i] = branchId
		i++

		// abort if we could not unwrap the Branch (should never happen)
		branch := cachedBranch.Unwrap()
		if branch == nil {
			cachedBranch.Release()

			err = fmt.Errorf("failed to unwrap brach '%s'", branchId)

			continue
		}

		if branch.IsAggregated() {
			aggregatedBranchConflictParents[branchId] = cachedBranch

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

	aggregatedBranchId = branchManager.generateAggregatedBranchId(aggregatedBranchConflictParents)

	return
}

func (branchManager *BranchManager) generateAggregatedBranchId(aggregatedBranches CachedBranches) BranchId {
	counter := 0
	branchIds := make([]BranchId, len(aggregatedBranches))
	for branchId, cachedBranch := range aggregatedBranches {
		branchIds[counter] = branchId

		counter++

		cachedBranch.Release()
	}

	sort.Slice(branchIds, func(i, j int) bool {
		for k := 0; k < len(branchIds[k]); k++ {
			if branchIds[i][k] < branchIds[j][k] {
				return true
			} else if branchIds[i][k] > branchIds[j][k] {
				return false
			}
		}

		return false
	})

	marshalUtil := marshalutil.New(BranchIdLength * len(branchIds))
	for _, branchId := range branchIds {
		marshalUtil.WriteBytes(branchId.Bytes())
	}

	return blake2b.Sum256(marshalUtil.Bytes())
}

func (branchManager *BranchManager) collectClosestConflictAncestors(branch *Branch, closestConflictAncestors CachedBranches) (err error) {
	// initialize stack
	stack := list.New()
	for _, parentRealityId := range branch.ParentBranches() {
		stack.PushBack(parentRealityId)
	}

	// work through stack
	processedBranches := make(map[BranchId]types.Empty)
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchId := stack.Front().Value.(BranchId)

			// abort if the parent has been processed already
			if _, branchProcessed := processedBranches[parentBranchId]; branchProcessed {
				return nil
			}
			processedBranches[parentBranchId] = types.Void

			// load parent branch from database
			cachedParentBranch := branchManager.GetBranch(parentBranchId)

			// abort if the parent branch could not be found (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to load branch '%s'", parentBranchId)
			}

			// if the parent Branch is not aggregated, then we have found the closest conflict ancestor
			if !parentBranch.IsAggregated() {
				closestConflictAncestors[parentBranchId] = cachedParentBranch

				return nil
			}

			// queue parents for additional check (recursion)
			for _, parentRealityId := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityId)
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
func (branchManager *BranchManager) findDeepestCommonAncestorBranches(branches ...BranchId) (result CachedBranches, err error) {
	result = make(CachedBranches)

	processedBranches := make(map[BranchId]types.Empty)
	for _, branchId := range branches {
		err = func() error {
			// continue, if we have processed this branch already
			if _, exists := processedBranches[branchId]; exists {
				return nil
			}
			processedBranches[branchId] = types.Void

			// load branch from objectstorage
			cachedBranch := branchManager.GetBranch(branchId)

			// abort if we could not load the CachedBranch
			branch := cachedBranch.Unwrap()
			if branch == nil {
				cachedBranch.Release()

				return fmt.Errorf("could not load branch '%s'", branchId)
			}

			// check branches position relative to already aggregated branches
			for aggregatedBranchId, cachedAggregatedBranch := range result {
				// abort if we can not load the branch
				aggregatedBranch := cachedAggregatedBranch.Unwrap()
				if aggregatedBranch == nil {
					return fmt.Errorf("could not load branch '%s'", aggregatedBranchId)
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
					delete(result, aggregatedBranchId)
					cachedAggregatedBranch.Release()

					result[branchId] = cachedBranch

					return nil
				}
			}

			// store the branch as a new aggregate candidate if it was not found to be in any relation with the already
			// aggregated ones.
			result[branchId] = cachedBranch

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
	if ancestor.Id() == descendant.Id() {
		return true, nil
	}

	ancestorBranches, err := branchManager.getAncestorBranches(descendant)
	if err != nil {
		return
	}

	ancestorBranches.Consume(func(ancestorOfDescendant *Branch) {
		if ancestorOfDescendant.Id() == ancestor.Id() {
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
	for _, parentRealityId := range branch.ParentBranches() {
		stack.PushBack(parentRealityId)
	}

	// work through stack
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchId := stack.Front().Value.(BranchId)

			// abort if the parent has been processed already
			if _, branchProcessed := ancestorBranches[parentBranchId]; branchProcessed {
				return nil
			}

			// load parent branch from database
			cachedParentBranch := branchManager.GetBranch(parentBranchId)

			// abort if the parent branch could not be founds (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to unwrap branch '%s'", parentBranchId)
			}

			// store parent branch in result
			ancestorBranches[parentBranchId] = cachedParentBranch

			// queue parents for additional check (recursion)
			for _, parentRealityId := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityId)
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
