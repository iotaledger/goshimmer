package ledgerstate

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/stack"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

// region BranchDAG ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions of the ledger state that exist in the tangle.
type BranchDAG struct {
	// Events is a container for all of the BranchDAG related events.
	Events *BranchDAGEvents

	branchStorage         *objectstorage.ObjectStorage
	childBranchStorage    *objectstorage.ObjectStorage
	conflictStorage       *objectstorage.ObjectStorage
	conflictMemberStorage *objectstorage.ObjectStorage
	shutdownOnce          sync.Once
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(store kvstore.KVStore) (newBranchDAG *BranchDAG) {
	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)
	newBranchDAG = &BranchDAG{
		Events:                NewBranchDAGEvents(),
		branchStorage:         osFactory.New(PrefixBranchStorage, BranchFromObjectStorage, branchStorageOptions...),
		childBranchStorage:    osFactory.New(PrefixChildBranchStorage, ChildBranchFromObjectStorage, childBranchStorageOptions...),
		conflictStorage:       osFactory.New(PrefixConflictStorage, ConflictFromObjectStorage, conflictStorageOptions...),
		conflictMemberStorage: osFactory.New(PrefixConflictMemberStorage, ConflictMemberFromObjectStorage, conflictMemberStorageOptions...),
	}
	newBranchDAG.init()

	return
}

// CreateConflictBranch retrieves the ConflictBranch that corresponds to the given details. It automatically creates and
// updates the ConflictBranch according to the new details if necessary.
func (b *BranchDAG) CreateConflictBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (cachedConflictBranch *CachedBranch, newBranchCreated bool, err error) {
	normalizedParentBranchIDs, err := b.normalizeBranches(parentBranchIDs)
	if err != nil {
		err = xerrors.Errorf("failed to normalize parent Branches: %w", err)
		return
	}

	cachedConflictBranch, newBranchCreated, err = b.createConflictBranchFromNormalizedParentBranchIDs(branchID, normalizedParentBranchIDs, conflictIDs)
	return
}

// UpdateConflictBranchParents changes the parents of a ConflictBranch (also updating the references of the
// ChildBranches).
func (b *BranchDAG) UpdateConflictBranchParents(conflictBranchID BranchID, newParentBranchIDs BranchIDs) (err error) {
	cachedConflictBranch := b.Branch(conflictBranchID)
	defer cachedConflictBranch.Release()

	conflictBranch, err := cachedConflictBranch.UnwrapConflictBranch()
	if err != nil {
		err = xerrors.Errorf("failed to unwrap ConflictBranch: %w", err)
		return
	}
	if conflictBranch == nil {
		err = xerrors.Errorf("failed to unwrap ConflictBranch: %w", cerrors.ErrFatal)
		return
	}

	newParentBranchIDs, err = b.normalizeBranches(newParentBranchIDs)
	if err != nil {
		err = xerrors.Errorf("failed to normalize new parent BranchIDs: %w", err)
		return
	}

	oldParentBranchIDs := conflictBranch.Parents()
	for oldParentBranchID := range oldParentBranchIDs {
		if _, exists := newParentBranchIDs[conflictBranchID]; !exists {
			b.childBranchStorage.Delete(NewChildBranch(oldParentBranchID, conflictBranchID, ConflictBranchType).ObjectStorageKey())
		}
	}

	for newParentBranchID := range newParentBranchIDs {
		if _, exists := oldParentBranchIDs[newParentBranchID]; !exists {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(newParentBranchID, conflictBranchID, ConflictBranchType)); stored {
				cachedChildBranch.Release()
			}
		}
	}

	conflictBranch.SetParents(newParentBranchIDs)

	return
}

// AggregateBranches retrieves the AggregatedBranch that corresponds to the given BranchIDs. It automatically creates
// the AggregatedBranch if it didn't exist, yet.
func (b *BranchDAG) AggregateBranches(branchIDS BranchIDs) (cachedAggregatedBranch *CachedBranch, newBranchCreated bool, err error) {
	normalizedBranchIDs, err := b.normalizeBranches(branchIDS)
	if err != nil {
		err = xerrors.Errorf("failed to normalize Branches: %w", err)
		return
	}

	cachedAggregatedBranch, newBranchCreated, err = b.aggregateNormalizedBranches(normalizedBranchIDs)
	return
}

// SetBranchLiked sets the liked flag of the given Branch. It returns true if the value has been updated or an error if
// it failed.
func (b *BranchDAG) SetBranchLiked(branchID BranchID, liked bool) (modified bool, err error) {
	return b.setBranchLiked(b.Branch(branchID), liked)
}

// SetBranchMonotonicallyLiked sets the monotonically liked flag of the given Branch. It returns true if the value has
// been updated or an error if failed.
func (b *BranchDAG) SetBranchMonotonicallyLiked(branchID BranchID, monotonicallyLiked bool) (modified bool, err error) {
	return b.setBranchMonotonicallyLiked(b.Branch(branchID), monotonicallyLiked)
}

// SetBranchFinalized sets the finalized flag of the given Branch. It returns true if the value has been updated or an
// error if it failed.
func (b *BranchDAG) SetBranchFinalized(branchID BranchID, finalized bool) (modified bool, err error) {
	return b.setBranchFinalized(b.Branch(branchID), finalized)
}

// MergeToMaster merges a confirmed Branch with the MasterBranch to clean up the BranchDAG. It reorganizes existing
// ChildBranches by adjusting their parents accordingly.
func (b *BranchDAG) MergeToMaster(branchID BranchID) (movedBranches map[BranchID]BranchID, err error) {
	movedBranches = make(map[BranchID]BranchID)

	// load Branch
	cachedBranch := b.Branch(branchID)
	defer cachedBranch.Release()

	// unwrap ConflictBranch
	conflictBranch, err := cachedBranch.UnwrapConflictBranch()
	if err != nil {
		err = xerrors.Errorf("tried to merge non-ConflictBranch with %s to Master: %w", branchID, err)
		return
	} else if conflictBranch == nil {
		err = xerrors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
		return
	}

	// abort if the Branch is not Confirmed
	if conflictBranch.InclusionState() != Confirmed {
		err = xerrors.Errorf("tried to merge non-confirmed Branch with %s to Master: %w", branchID, cerrors.ErrFatal)
		return
	}

	// abort if the Branch is not at the bottom of the BranchDAG
	parentBranches := conflictBranch.Parents()
	if _, masterBranchIsParent := parentBranches[MasterBranchID]; len(parentBranches) != 1 || !masterBranchIsParent {
		err = xerrors.Errorf("tried to merge Branch with %s to Master that is not at the bottom of the BranchDAG: %w", branchID, err)
		return
	}

	// remove merged Branch
	conflictBranch.Delete()
	movedBranches[conflictBranch.ID()] = MasterBranchID

	// load ChildBranch references
	cachedChildBranchReferences := b.ChildBranches(branchID)
	defer cachedChildBranchReferences.Release()

	// reorganize ChildBranches
	for _, cachedChildBranchReference := range cachedChildBranchReferences {
		childBranchReference := cachedChildBranchReference.Unwrap()
		if childBranchReference == nil {
			err = xerrors.Errorf("failed to load ChildBranch reference: %w", cerrors.ErrFatal)
			return
		}

		switch childBranchReference.ChildBranchType() {
		case AggregatedBranchType:
			// load referenced ChildBranch
			cachedChildBranch := b.Branch(childBranchReference.ChildBranchID())
			childBranch, unwrapErr := cachedChildBranch.UnwrapAggregatedBranch()
			if unwrapErr != nil {
				cachedChildBranch.Release()
				err = xerrors.Errorf("failed to load AggregatedBranch with %s: %w", cachedChildBranch.ID(), unwrapErr)
				return
			}

			// remove merged Branch from parents
			parentBranches = childBranch.Parents()
			delete(parentBranches, branchID)

			if len(parentBranches) == 1 {
				for parentBranchID := range parentBranches {
					movedBranches[childBranch.ID()] = parentBranchID

					b.childBranchStorage.Delete(byteutils.ConcatBytes(parentBranchID.Bytes(), childBranch.ID().Bytes()))
				}
			} else {
				cachedNewAggregatedBranch, _, newAggregatedBranchErr := b.AggregateBranches(parentBranches)
				if newAggregatedBranchErr != nil {
					err = xerrors.Errorf("failed to retrieve AggregatedBranch: %w", newAggregatedBranchErr)
					return
				}
				cachedNewAggregatedBranch.Release()

				movedBranches[childBranch.ID()] = cachedNewAggregatedBranch.ID()

				for parentBranchID := range parentBranches {
					b.childBranchStorage.Delete(byteutils.ConcatBytes(parentBranchID.Bytes(), childBranch.ID().Bytes()))
				}
			}

			childBranch.Delete()

			// release referenced ChildBranch
			cachedChildBranch.Release()
		case ConflictBranchType:
			// load referenced ChildBranch
			cachedChildBranch := b.Branch(childBranchReference.ChildBranchID())
			childBranch, unwrapErr := cachedChildBranch.UnwrapConflictBranch()
			if unwrapErr != nil {
				cachedChildBranch.Release()
				err = xerrors.Errorf("failed to load ConflictBranch with %s: %w", cachedChildBranch.ID(), unwrapErr)
				return
			}

			// replace pointer to current branch with pointer to master
			parents := childBranch.Parents()
			delete(parents, branchID)
			parents[MasterBranchID] = types.Void
			childBranch.SetParents(parents)

			// release referenced ChildBranch
			cachedChildBranch.Release()
		}

		// remove ChildBranch reference
		childBranchReference.Delete()
	}

	// update ConflictMembers to not contain the merged Branch
	for conflictID := range conflictBranch.Conflicts() {
		b.unregisterConflictMember(conflictID, branchID)
	}

	return
}

// Branch retrieves the Branch with the given BranchID from the object storage.
func (b *BranchDAG) Branch(branchID BranchID) (cachedBranch *CachedBranch) {
	return &CachedBranch{CachedObject: b.branchStorage.Load(branchID.Bytes())}
}

// ChildBranches loads the references to the ChildBranches of the given Branch from the object storage.
func (b *BranchDAG) ChildBranches(branchID BranchID) (cachedChildBranches CachedChildBranches) {
	cachedChildBranches = make(CachedChildBranches, 0)
	b.childBranchStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedChildBranches = append(cachedChildBranches, &CachedChildBranch{CachedObject: cachedObject})

		return true
	}, branchID.Bytes())

	return
}

// Conflict loads a Conflict from the object storage.
func (b *BranchDAG) Conflict(conflictID ConflictID) *CachedConflict {
	return &CachedConflict{CachedObject: b.conflictStorage.Load(conflictID.Bytes())}
}

// ConflictMembers loads the referenced ConflictMembers of a Conflict from the object storage.
func (b *BranchDAG) ConflictMembers(conflictID ConflictID) (cachedConflictMembers CachedConflictMembers) {
	cachedConflictMembers = make(CachedConflictMembers, 0)
	b.conflictMemberStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedConflictMembers = append(cachedConflictMembers, &CachedConflictMember{CachedObject: cachedObject})

		return true
	}, conflictID.Bytes())

	return
}

// BranchIDsContainRejectedBranch is an utility function that checks if the given BranchIDs contain a Rejected
// Branch. It returns the BranchID of the first Rejected Branch that it finds.
func (b *BranchDAG) BranchIDsContainRejectedBranch(branchIDs BranchIDs) (rejected bool, rejectedBranchID BranchID) {
	for rejectedBranch := range branchIDs {
		if !b.Branch(rejectedBranch).Consume(func(branch Branch) {
			rejected = branch.InclusionState() == Rejected
		}) {
			panic(fmt.Sprintf("failed to load Branch with %s", rejectedBranch))
		}

		if rejected {
			return
		}
	}

	rejectedBranchID = UndefinedBranchID
	return
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (b *BranchDAG) Prune() (err error) {
	for _, storage := range []*objectstorage.ObjectStorage{
		b.branchStorage,
		b.childBranchStorage,
		b.conflictStorage,
		b.conflictMemberStorage,
	} {
		if err = storage.Prune(); err != nil {
			err = xerrors.Errorf("failed to prune the object storage (%v): %w", err, cerrors.ErrFatal)
			return
		}
	}

	b.init()

	return
}

// Shutdown shuts down the BranchDAG and persists its state.
func (b *BranchDAG) Shutdown() {
	b.shutdownOnce.Do(func() {
		b.branchStorage.Shutdown()
		b.childBranchStorage.Shutdown()
		b.conflictStorage.Shutdown()
		b.conflictMemberStorage.Shutdown()
	})
}

// init is an internal utility function that initializes the BranchDAG by creating the root of the DAG (MasterBranch).
func (b *BranchDAG) init() {
	cachedMasterBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(MasterBranchID, nil, nil))
	if stored {
		(&CachedBranch{CachedObject: cachedMasterBranch}).Consume(func(branch Branch) {
			branch.SetLiked(true)
			branch.SetMonotonicallyLiked(true)
			branch.SetFinalized(true)
			branch.SetInclusionState(Confirmed)
		})
	}

	cachedRejectedBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(InvalidBranchID, nil, nil))
	if stored {
		(&CachedBranch{CachedObject: cachedRejectedBranch}).Consume(func(branch Branch) {
			branch.SetLiked(false)
			branch.SetMonotonicallyLiked(false)
			branch.SetFinalized(true)
			branch.SetInclusionState(Rejected)
		})
	}

	cachedLazyBookedConflictsBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(LazyBookedConflictsBranchID, nil, nil))
	if stored {
		(&CachedBranch{CachedObject: cachedLazyBookedConflictsBranch}).Consume(func(branch Branch) {
			branch.SetLiked(false)
			branch.SetMonotonicallyLiked(false)
			branch.SetFinalized(true)
			branch.SetInclusionState(Rejected)
		})
	}
}

// resolveConflictBranchIDs is an internal utility function that returns the BranchIDs of the ConflictBranches that the
// given Branches represent by resolving AggregatedBranches to their corresponding ConflictBranches.
func (b *BranchDAG) resolveConflictBranchIDs(branchIDs BranchIDs) (conflictBranchIDs BranchIDs, err error) {
	// initialize return variable
	conflictBranchIDs = make(BranchIDs)

	// iterate through parameters and collect the conflict branches
	seenBranches := set.New()
	for branchID := range branchIDs {
		// abort if branch was processed already
		if !seenBranches.Add(branchID) {
			continue
		}

		// process branch or abort if it can not be found
		if !b.Branch(branchID).Consume(func(branch Branch) {
			switch branch.Type() {
			case ConflictBranchType:
				conflictBranchIDs[branch.ID()] = types.Void
			case AggregatedBranchType:
				for parentBranchID := range branch.Parents() {
					conflictBranchIDs[parentBranchID] = types.Void
				}
			}
		}) {
			err = xerrors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
			return
		}
	}

	return
}

// normalizeBranches is an internal utility function that takes a list of BranchIDs and returns the BranchIDS of the
// most special ConflictBranches that the given BranchIDs represent. It returns an error if the Branches are conflicting
// or any other unforeseen error occurred.
func (b *BranchDAG) normalizeBranches(branchIDs BranchIDs) (normalizedBranches BranchIDs, err error) {
	// retrieve conflict branches and abort if we faced an error
	conflictBranches, err := b.resolveConflictBranchIDs(branchIDs)
	if err != nil {
		err = xerrors.Errorf("failed to resolve ConflictBranchIDs: %w", err)
		return
	}

	// return if we are done
	if len(conflictBranches) == 1 {
		normalizedBranches = conflictBranches
		return
	}

	// return the master branch if the list of conflict branches is empty
	if len(conflictBranches) == 0 {
		return BranchIDs{MasterBranchID: types.Void}, nil
	}

	// introduce iteration variables
	traversedBranches := set.New()
	seenConflictSets := set.New()
	parentsToCheck := stack.New()

	// checks if branches are conflicting and queues parents to be checked
	checkConflictsAndQueueParents := func(currentBranch Branch) {
		currentConflictBranch, typeCastOK := currentBranch.(*ConflictBranch)
		if !typeCastOK {
			err = xerrors.Errorf("failed to type cast Branch with %s to ConflictBranch: %w", currentBranch.ID(), cerrors.ErrFatal)
			return
		}

		// abort if branch was traversed already
		if !traversedBranches.Add(currentConflictBranch.ID()) {
			return
		}

		// return error if conflict set was seen twice
		for conflictSetID := range currentConflictBranch.Conflicts() {
			if !seenConflictSets.Add(conflictSetID) {
				err = xerrors.Errorf("combined Branches are conflicting: %w", ErrInvalidStateTransition)
				return
			}
		}

		// queue parents to be checked when traversing ancestors
		for parentBranchID := range currentConflictBranch.Parents() {
			parentsToCheck.Push(parentBranchID)
		}
	}

	// create normalized branch candidates (check their conflicts and queue parent checks)
	normalizedBranches = make(BranchIDs)
	for conflictBranchID := range conflictBranches {
		// add branch to the candidates of normalized branches
		normalizedBranches[conflictBranchID] = types.Void

		// check branch and queue parents
		if !b.Branch(conflictBranchID).Consume(checkConflictsAndQueueParents) {
			err = xerrors.Errorf("failed to load Branch with %s: %w", conflictBranchID, cerrors.ErrFatal)
			return
		}

		// abort if we faced an error
		if err != nil {
			return
		}
	}

	// remove ancestors from the candidates
	for !parentsToCheck.IsEmpty() {
		// retrieve parent branch ID from stack
		parentBranchID := parentsToCheck.Pop().(BranchID)

		// remove ancestor from normalized candidates
		delete(normalizedBranches, parentBranchID)

		// check branch, queue parents and abort if we faced an error
		if !b.Branch(parentBranchID).Consume(checkConflictsAndQueueParents) {
			err = xerrors.Errorf("failed to load Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
			return
		}

		// abort if we faced an error
		if err != nil {
			return
		}
	}

	return
}

// createConflictBranchFromNormalizedParentBranchIDs is an internal utility function that retrieves the ConflictBranch
// that corresponds to the given details. It automatically creates and updates the ConflictBranch according to the new
// details if necessary.
func (b *BranchDAG) createConflictBranchFromNormalizedParentBranchIDs(branchID BranchID, normalizedParentBranchIDs BranchIDs, conflictIDs ConflictIDs) (cachedConflictBranch *CachedBranch, newBranchCreated bool, err error) {
	// create or load the branch
	cachedConflictBranch = &CachedBranch{
		CachedObject: b.branchStorage.ComputeIfAbsent(branchID.Bytes(),
			func(key []byte) objectstorage.StorableObject {
				newBranch := NewConflictBranch(branchID, normalizedParentBranchIDs, conflictIDs)
				newBranch.Persist()
				newBranch.SetModified()

				newBranchCreated = true

				return newBranch
			},
		),
	}
	branch := cachedConflictBranch.Unwrap()
	if branch == nil {
		err = xerrors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
		return
	}

	// type cast to ConflictBranch
	conflictBranch, typeCastOK := branch.(*ConflictBranch)
	if !typeCastOK {
		err = xerrors.Errorf("failed to type cast Branch with %s to ConflictBranch: %w", branchID, cerrors.ErrFatal)
		return
	}

	// register references
	switch true {
	case newBranchCreated:
		// store child references
		for parentBranchID := range normalizedParentBranchIDs {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID, ConflictBranchType)); stored {
				cachedChildBranch.Release()
			}
		}

		// store ConflictMember references
		for conflictID := range conflictIDs {
			b.registerConflictMember(conflictID, branchID)
		}
	default:
		// store new ConflictMember references
		for conflictID := range conflictIDs {
			if conflictBranch.AddConflict(conflictID) {
				b.registerConflictMember(conflictID, branchID)
			}
		}
	}

	return
}

// aggregateNormalizedBranches is an internal utility function that retrieves the AggregatedBranch that corresponds to
// the given normalized BranchIDs. It automatically creates the AggregatedBranch if it didn't exist, yet.
func (b *BranchDAG) aggregateNormalizedBranches(normalizedBranchIDs BranchIDs) (cachedAggregatedBranch *CachedBranch, newBranchCreated bool, err error) {
	if len(normalizedBranchIDs) == 1 {
		for firstBranchID := range normalizedBranchIDs {
			cachedAggregatedBranch = b.Branch(firstBranchID)
			return
		}
	}

	aggregatedBranch := NewAggregatedBranch(normalizedBranchIDs)
	cachedAggregatedBranch = &CachedBranch{CachedObject: b.branchStorage.ComputeIfAbsent(aggregatedBranch.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		newBranchCreated = true

		aggregatedBranch.Persist()
		aggregatedBranch.SetModified()

		return aggregatedBranch
	})}

	if newBranchCreated {
		for parentBranchID := range normalizedBranchIDs {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, aggregatedBranch.ID(), AggregatedBranchType)); stored {
				cachedChildBranch.Release()
			}
		}

		for normalizedBranchID := range normalizedBranchIDs {
			if !b.Branch(normalizedBranchID).Consume(func(normalizedBranch Branch) {
				if likedErr := b.updateLikedOfAggregatedBranch(cachedAggregatedBranch.Retain(), normalizedBranch.Liked()); likedErr != nil {
					err = xerrors.Errorf("failed to update liked flag of newly added AggregatedBranch with %s: %w", aggregatedBranch.ID(), likedErr)
					return
				}
				if _, monotonicallyLikedErr := b.updateMonotonicallyLikedStatus(cachedAggregatedBranch.ID(), normalizedBranch.MonotonicallyLiked()); monotonicallyLikedErr != nil {
					err = xerrors.Errorf("failed to update monotonically liked flag of newly added AggregatedBranch with %s: %w", aggregatedBranch.ID(), monotonicallyLikedErr)
					return
				}
				if finalizedErr := b.updateFinalizedOfAggregatedBranch(cachedAggregatedBranch.Retain(), normalizedBranch.Finalized()); finalizedErr != nil {
					err = xerrors.Errorf("failed to update finalized flag of newly added AggregatedBranch with %s: %w", aggregatedBranch.ID(), finalizedErr)
					return
				}
				if inclusionStateErr := b.updateInclusionState(aggregatedBranch.ID(), normalizedBranch.InclusionState()); inclusionStateErr != nil {
					err = xerrors.Errorf("failed to update inclusion state of newly added AggregatedBranch with %s: %w", aggregatedBranch.ID(), inclusionStateErr)
					return
				}
			}) {
				err = xerrors.Errorf("failed to load parent Branch with %s: %w", normalizedBranchID, cerrors.ErrFatal)
			}

			return
		}

		err = xerrors.Errorf("failed to load parent Branch: %w", cerrors.ErrFatal)
	}

	return
}

// registerConflictMember is an internal utility function that removes the ConflictMember references of a Branch
// belonging to a given Conflict. It automatically creates the Conflict if it doesn't exist, yet.
func (b *BranchDAG) unregisterConflictMember(conflictID ConflictID, branchID BranchID) {
	(&CachedConflict{CachedObject: b.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) objectstorage.StorableObject {
		newConflict := NewConflict(conflictID)
		newConflict.Persist()
		newConflict.SetModified()

		return newConflict
	})}).Consume(func(conflict *Conflict) {
		if b.conflictMemberStorage.DeleteIfPresent(NewConflictMember(conflictID, branchID).ObjectStorageKey()) {
			conflict.DecreaseMemberCount()
		}
	})
}

// registerConflictMember is an internal utility function that creates the ConflictMember references of a Branch
// belonging to a given Conflict. It automatically creates the Conflict if it doesn't exist, yet.
func (b *BranchDAG) registerConflictMember(conflictID ConflictID, branchID BranchID) {
	(&CachedConflict{CachedObject: b.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) objectstorage.StorableObject {
		newConflict := NewConflict(conflictID)
		newConflict.Persist()
		newConflict.SetModified()

		return newConflict
	})}).Consume(func(conflict *Conflict) {
		if cachedConflictMember, stored := b.conflictMemberStorage.StoreIfAbsent(NewConflictMember(conflictID, branchID)); stored {
			conflict.IncreaseMemberCount()

			cachedConflictMember.Release()
		}
	})
}

// setBranchLiked updates the liked flag of the given Branch. It returns true if the value has been updated or an error
// if it failed.
func (b *BranchDAG) setBranchLiked(cachedBranch *CachedBranch, liked bool) (modified bool, err error) {
	// release the CachedBranch when we are done
	defer cachedBranch.Release()

	// unwrap ConflictBranch
	conflictBranch, err := cachedBranch.UnwrapConflictBranch()
	if err != nil {
		err = xerrors.Errorf("failed to load ConflictBranch with %s: %w", cachedBranch.ID(), cerrors.ErrFatal)
		return
	}

	// execute case dependent logic
	switch liked {
	case true:
		// iterate through all Conflicts of the current Branch and set their ConflictMembers to be not liked
		for conflictID := range conflictBranch.Conflicts() {
			// iterate through all ConflictMembers and set them to not liked
			cachedConflictMembers := b.ConflictMembers(conflictID)
			for _, cachedConflictMember := range cachedConflictMembers {
				// unwrap the ConflictMember
				conflictMember := cachedConflictMember.Unwrap()
				if conflictMember == nil {
					cachedConflictMembers.Release()
					err = xerrors.Errorf("failed to load ConflictMember of %s: %w", conflictID, cerrors.ErrFatal)
					return
				}

				// skip the current Branch
				if conflictMember.BranchID() == conflictBranch.ID() {
					continue
				}

				// update the other ConflictMembers to be not liked
				if _, err = b.setBranchLiked(b.Branch(conflictMember.BranchID()), false); err != nil {
					cachedConflictMembers.Release()
					err = xerrors.Errorf("failed to propagate liked changes to other ConflictMembers: %w", err)
					return
				}
			}
			cachedConflictMembers.Release()
		}

		// abort if the branch was liked already
		if modified = conflictBranch.SetLiked(true); !modified {
			return
		}

		// trigger event
		b.Events.BranchLiked.Trigger(NewBranchDAGEvent(cachedBranch))

		// update the liked status of the future cone (it only effects the AggregatedBranches)
		if err = b.updateLikedOfAggregatedChildBranches(conflictBranch.ID(), true); err != nil {
			err = xerrors.Errorf("failed to update liked status of future cone of Branch with %s: %w", conflictBranch.ID(), err)
			return
		}

		// update the liked status of the future cone (if necessary)
		if _, err = b.updateMonotonicallyLikedStatus(conflictBranch.ID(), true); err != nil {
			err = xerrors.Errorf("failed to update liked status of future cone of Branch with %s: %w", conflictBranch.ID(), err)
			return
		}
	case false:
		// set the branch to be not liked
		if modified = conflictBranch.SetLiked(false); modified {
			// trigger event
			b.Events.BranchDisliked.Trigger(NewBranchDAGEvent(cachedBranch))

			// update the liked status of the future cone (it only affect the AggregatedBranches)
			if propagationErr := b.updateLikedOfAggregatedChildBranches(conflictBranch.ID(), false); propagationErr != nil {
				err = xerrors.Errorf("failed to propagate liked changes to AggregatedBranches: %w", propagationErr)
				return
			}

			// update the liked status of the future cone (if necessary)
			if _, propagationErr := b.updateMonotonicallyLikedStatus(conflictBranch.ID(), false); propagationErr != nil {
				err = xerrors.Errorf("failed to update liked status of future cone of Branch with %s: %w", conflictBranch.ID(), propagationErr)
				return
			}
		}
	}

	return
}

// setBranchMonotonicallyLiked is an internal utility function that updates the liked status of a Branch. Since a Branch can only be
// liked if its parents are also liked, we automatically update them as well.
func (b *BranchDAG) setBranchMonotonicallyLiked(cachedBranch *CachedBranch, liked bool) (modified bool, err error) {
	// release the CachedBranch when we are done
	defer cachedBranch.Release()

	// unwrap Branch
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = xerrors.Errorf("failed to load Branch with %s: %w", cachedBranch.ID(), cerrors.ErrFatal)
		return
	}

	// process AggregatedBranches differently (they don't have their own "liked flag" but depend on their parents).
	if branch.Type() == AggregatedBranchType {
		// disliking an AggregatedBranch is too "unspecific" - we need to know which one of its parents caused the
		// dislike
		if !liked {
			err = xerrors.Errorf("Branch of type AggregatedBranchType can not be disliked (status depends on its parents): %w", cerrors.ErrFatal)
			return
		}

		// iterate through parents and like them instead (the change will trickle through)
		for parentBranchID := range branch.Parents() {
			parentBranchModified, parentBranchErr := b.setBranchMonotonicallyLiked(b.Branch(parentBranchID), true)
			if parentBranchErr != nil {
				err = xerrors.Errorf("failed to set liked status of parent Branch with %s: %w", parentBranchID, parentBranchErr)
				return
			}
			modified = modified || parentBranchModified
		}

		return
	}

	// execute case dependent logic
	switch liked {
	case true:
		// start with liking the parents (like propagates from past to present)
		for parentBranchID := range branch.Parents() {
			if _, err = b.setBranchMonotonicallyLiked(b.Branch(parentBranchID), true); err != nil {
				err = xerrors.Errorf("failed to set liked status of parent Branch with %s: %w", parentBranchID, err)
				return
			}
		}

		// set all conflict members to be not liked (and consequently also not liked) - there can only be one
		// liked Branch per Conflict
		for conflictID := range branch.(*ConflictBranch).Conflicts() {
			b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
				// skip the current Branch
				if conflictMember.BranchID() == branch.ID() {
					return
				}

				// set other ConflictMembers to not be liked anymore - there can only be one liked Branch per
				// Conflict
				_, err = b.setBranchLiked(b.Branch(conflictMember.BranchID()), false)
				if err != nil {
					err = xerrors.Errorf("failed to update liked status of parent Branch with %s: %w", conflictMember.BranchID(), err)
					return
				}
			})
		}

		// trigger event if we changed the flag
		if branch.SetLiked(true) {
			b.Events.BranchLiked.Trigger(NewBranchDAGEvent(cachedBranch))
		}

		// update the liked status of the future cone (it only effects the AggregatedBranches)
		if err = b.updateLikedOfAggregatedChildBranches(branch.ID(), true); err != nil {
			err = xerrors.Errorf("failed to update liked status of future cone of Branch with %s: %w", branch.ID(), err)
			return
		}

		// update the liked status and determine the modified flag
		if modified, err = b.updateMonotonicallyLikedStatus(branch.ID(), true); err != nil {
			err = xerrors.Errorf("failed to update liked status of Branch with %s: %w", branch.ID(), err)
			return
		}
	case false:
		// abort if the Branch is already not liked
		if !branch.SetLiked(false) {
			return
		}

		// trigger event
		b.Events.BranchDisliked.Trigger(NewBranchDAGEvent(cachedBranch))

		// update the like status and set the modified flag
		if modified, err = b.updateMonotonicallyLikedStatus(branch.ID(), false); err != nil {
			err = xerrors.Errorf("failed to update liked status of future cone of Branch with %s: %w", branch.ID(), err)
			return
		}
	}

	return
}

// setBranchFinalized updates the finalized flag of the given Branch. It returns true if the value has been updated or
// an error if it failed.
func (b *BranchDAG) setBranchFinalized(cachedBranch *CachedBranch, finalized bool) (modified bool, err error) {
	// release CachedObject when we are done
	defer cachedBranch.Release()

	// unwrap ConflictBranch
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = xerrors.Errorf("failed to load ConflictBranch with %s: %w", cachedBranch.ID(), cerrors.ErrFatal)
		return
	}

	// process AggregatedBranches differently (they don't have their own "finalized flag" but depend on their parents).
	if branch.Type() == AggregatedBranchType {
		// unfinalizing an AggregatedBranch is too "unspecific" - we need to know which one of its parents caused the
		// unfinalize
		if !finalized {
			err = xerrors.Errorf("Branch of type AggregatedBranchType can not be unfinalized (status depends on its parents): %w", cerrors.ErrFatal)
			return
		}

		// iterate through parents and like them instead (the change will trickle through)
		for parentBranchID := range branch.Parents() {
			parentBranchModified, parentBranchErr := b.setBranchFinalized(b.Branch(parentBranchID), true)
			if parentBranchErr != nil {
				err = xerrors.Errorf("failed to set finalized status of parent Branch with %s: %w", parentBranchID, parentBranchErr)
				return
			}
			modified = modified || parentBranchModified
		}

		return
	}

	// abort if the ConflictBranch was marked correctly already
	if modified = branch.SetFinalized(finalized); !modified {
		return
	}

	// execute case dependent logic
	switch finalized {
	case true:
		// trigger event
		b.Events.BranchFinalized.Trigger(NewBranchDAGEvent(cachedBranch))

		// propagate finalized update to aggregated ChildBranches
		if err = b.updateFinalizedOfAggregatedChildBranches(branch.ID(), true); err != nil {
			err = xerrors.Errorf("failed to update finalized status of future cone of Branch with %s: %w", branch.ID(), err)
			return
		}

		// execute case dependent logic
		switch branch.Liked() {
		case true:
			// iterate through all other Branches that belong to the same Conflict
			for conflictID := range branch.(*ConflictBranch).Conflicts() {
				b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
					// skip the branch which just got finalized
					if conflictMember.BranchID() == branch.ID() {
						return
					}

					// set other branches of same conflict to also be finalized (there can only be 1 liked, finalized
					// Branch in every Conflict)
					_, err = b.setBranchFinalized(b.Branch(conflictMember.BranchID()), true)
					if err != nil {
						err = xerrors.Errorf("failed to set other Branches of the same Conflict to also be finalized: %w", cerrors.ErrFatal)
						return
					}
				})
			}

			// update InclusionState of future cone
			if err = b.updateInclusionState(branch.ID(), Confirmed); err != nil {
				err = xerrors.Errorf("failed to update InclusionState of future cone of Branch with %s: %w", branch.ID(), err)
				return
			}
		case false:
			// update InclusionState of future cone
			if err = b.updateInclusionState(branch.ID(), Rejected); err != nil {
				err = xerrors.Errorf("failed to update InclusionState of future cone of Branch with %s: %w", branch.ID(), err)
				return
			}
		}
	case false:
		// trigger event
		b.Events.BranchUnfinalized.Trigger(NewBranchDAGEvent(cachedBranch))

		// propagate finalized update to aggregated ChildBranches
		if err = b.updateFinalizedOfAggregatedChildBranches(branch.ID(), false); err != nil {
			err = xerrors.Errorf("failed to update finalized status of future cone of Branch with %s: %w", branch.ID(), err)
			return
		}

		// update InclusionState of future cone
		if err = b.updateInclusionState(branch.ID(), Pending); err != nil {
			err = xerrors.Errorf("failed to update InclusionState of future cone of Branch with %s: %w", branch.ID(), err)
			return
		}
	}

	return
}

// updateLikedOfAggregatedChildBranches updates the liked flag of the AggregatedBranches that are direct children
// of the Branch whose liked flag was updated.
func (b *BranchDAG) updateLikedOfAggregatedChildBranches(branchID BranchID, liked bool) (err error) {
	// initialize stack with children of type AggregatedBranch of the passed in Branch (we only update the liked
	// status of AggregatedBranches as their status entirely depends on their parents)
	branchStack := list.New()
	b.ChildBranches(branchID).Consume(func(childBranch *ChildBranch) {
		if childBranch.ChildBranchType() == AggregatedBranchType {
			branchStack.PushBack(childBranch.ChildBranchID())
		}
	})

	// iterate through stack
	for branchStack.Len() >= 1 {
		// retrieve first element from the stack
		currentEntry := branchStack.Front()
		branchStack.Remove(currentEntry)

		if err = b.updateLikedOfAggregatedBranch(b.Branch(currentEntry.Value.(BranchID)), liked); err != nil {
			err = xerrors.Errorf("failed to update liked flag of AggregatedBranch with %s: %w", currentEntry.Value.(BranchID), err)
			return
		}
	}

	return
}

// updateLikedOfAggregatedBranch is an internal utility function that updates the liked status of a single
// AggregatedBranch.
func (b *BranchDAG) updateLikedOfAggregatedBranch(currentCachedBranch *CachedBranch, liked bool) (err error) {
	// release current CachedBranch
	defer currentCachedBranch.Release()

	// unwrap current CachedBranch
	currentBranch, typeErr := currentCachedBranch.UnwrapAggregatedBranch()
	if typeErr != nil {
		err = xerrors.Errorf("failed to load AggregatedBranch with %s: %w", currentCachedBranch.ID(), typeErr)
		return
	}
	if currentBranch == nil {
		err = xerrors.Errorf("failed to load AggregatedBranch with %s: %w", currentCachedBranch.ID(), cerrors.ErrFatal)
		return
	}

	// execute case dependent logic
	switch liked {
	case true:
		// only continue if all parents are liked
		for parentBranchID := range currentBranch.Parents() {
			// load parent Branch
			cachedParentBranch := b.Branch(parentBranchID)

			// unwrap parent Branch
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()
				err = xerrors.Errorf("failed to load parent Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
				return
			}

			// abort if the parent Branch is not liked
			if !parentBranch.Liked() {
				cachedParentBranch.Release()
				return
			}

			// release parent CachedBranch
			cachedParentBranch.Release()
		}

		// trigger event if the value was changed
		if currentBranch.SetLiked(true) {
			b.Events.BranchLiked.Trigger(NewBranchDAGEvent(currentCachedBranch))
		}
	case false:
		// trigger event if the value was changed
		if currentBranch.SetLiked(false) {
			b.Events.BranchDisliked.Trigger(NewBranchDAGEvent(currentCachedBranch))
		}
	}

	return
}

// updateFinalizedOfAggregatedChildBranches updates the finalized state of the AggregatedBranches that are direct
// children of the given Branch.
func (b *BranchDAG) updateFinalizedOfAggregatedChildBranches(branchID BranchID, finalized bool) (err error) {
	// initialize stack with children of type AggregatedBranch of the passed in Branch (we only update the finalized
	// state of AggregatedBranches as it entirely depends on their parents)
	branchStack := list.New()
	b.ChildBranches(branchID).Consume(func(childBranch *ChildBranch) {
		if childBranch.ChildBranchType() == AggregatedBranchType {
			branchStack.PushBack(childBranch.ChildBranchID())
		}
	})

	// iterate through stack
	for branchStack.Len() >= 1 {
		// retrieve first element from the stack
		currentEntry := branchStack.Front()
		branchStack.Remove(currentEntry)

		if err = b.updateFinalizedOfAggregatedBranch(b.Branch(currentEntry.Value.(BranchID)), finalized); err != nil {
			err = xerrors.Errorf("failed to update finalized flag of AggregatedBranch with %s: %w", currentEntry.Value.(BranchID), err)
			return
		}
	}

	return
}

// updateFinalizedOfAggregatedBranch is an internal utility function that updates the finalized status of a single
// AggregatedBranch.
func (b *BranchDAG) updateFinalizedOfAggregatedBranch(currentCachedBranch *CachedBranch, finalized bool) (err error) {
	// release current CachedBranch
	defer currentCachedBranch.Release()

	// unwrap current CachedBranch
	currentBranch, typeErr := currentCachedBranch.UnwrapAggregatedBranch()
	if typeErr != nil {
		err = xerrors.Errorf("failed to load AggregatedBranch with %s: %w", currentCachedBranch.ID(), typeErr)
		return
	}
	if currentBranch == nil {
		err = xerrors.Errorf("failed to load AggregatedBranch with %s: %w", currentCachedBranch.ID(), cerrors.ErrFatal)
		return
	}

	// execute case dependent logic
	switch finalized {
	case true:
		// only continue if all parents are finalized
		for parentBranchID := range currentBranch.Parents() {
			// load parent Branch
			cachedParentBranch := b.Branch(parentBranchID)

			// unwrap parent Branch
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()
				err = xerrors.Errorf("failed to load parent Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
				return
			}

			// abort if the parent Branch is not finalized
			if !parentBranch.Finalized() {
				cachedParentBranch.Release()
				return
			}

			// release parent CachedBranch
			cachedParentBranch.Release()
		}

		// trigger event if the value was changed
		if currentBranch.SetFinalized(true) {
			b.Events.BranchFinalized.Trigger(NewBranchDAGEvent(currentCachedBranch))
		}
	case false:
		// trigger event if the value was changed
		if currentBranch.SetFinalized(false) {
			b.Events.BranchUnfinalized.Trigger(NewBranchDAGEvent(currentCachedBranch))
		}
	}

	return
}

// updateMonotonicallyLikedStatus is an internal utility function that updates the monotonically liked status of the
// given Branch (and its future cone) if it fulfills the conditions.
func (b *BranchDAG) updateMonotonicallyLikedStatus(branchID BranchID, monotonicallyLiked bool) (modified bool, err error) {
	// initialize stack for iteration
	branchStack := list.New()
	branchStack.PushBack(branchID)

	// iterate through stack
	initialBranch := true
ProcessStack:
	for branchStack.Len() >= 1 {
		// retrieve first element from the stack
		currentStackElement := branchStack.Front()
		branchStack.Remove(currentStackElement)

		// load Branch
		currentCachedBranch := b.Branch(currentStackElement.Value.(BranchID))

		// unwrap current CachedBranch
		currentBranch := currentCachedBranch.Unwrap()
		if currentBranch == nil {
			currentCachedBranch.Release()
			err = xerrors.Errorf("failed to load Branch with %s: %w", currentCachedBranch.ID(), cerrors.ErrFatal)
			return
		}

		// execute case dependent logic
		switch monotonicallyLiked {
		case true:
			// abort if the current Branch is not liked
			if !currentBranch.Liked() {
				currentCachedBranch.Release()
				continue
			}

			// abort if any parent Branch is not liked
			for parentBranchID := range currentBranch.Parents() {
				// load parent Branch
				cachedParentBranch := b.Branch(parentBranchID)

				// unwrap parent Branch
				parentBranch := cachedParentBranch.Unwrap()
				if parentBranch == nil {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					err = xerrors.Errorf("failed to load parent Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
					return
				}

				// abort if parent Branch is not liked
				if !parentBranch.MonotonicallyLiked() {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					continue ProcessStack
				}

				// release parent CachedBranch
				cachedParentBranch.Release()
			}

			// abort if the Branch is already liked
			if !currentBranch.SetMonotonicallyLiked(true) {
				currentCachedBranch.Release()
				continue
			}

			// set modified to true if we modified the initial branch
			if initialBranch {
				modified = true
				initialBranch = false
			}

			// trigger event
			b.Events.BranchMonotonicallyLiked.Trigger(NewBranchDAGEvent(currentCachedBranch))
		case false:
			// abort if the current Branch is disliked already
			if !currentBranch.SetMonotonicallyLiked(false) {
				currentCachedBranch.Release()
				continue
			}

			// set modified to true if we modified the initial branch
			if initialBranch {
				modified = true
				initialBranch = false
			}

			// trigger event
			b.Events.BranchMonotonicallyDisliked.Trigger(NewBranchDAGEvent(currentCachedBranch))
		}

		// iterate through ChildBranch references and queue found Branches for propagation
		cachedChildBranchReferences := b.ChildBranches(currentBranch.ID())
		for _, cachedChildBranchReference := range cachedChildBranchReferences {
			// unwrap ChildBranch reference
			childBranchReference := cachedChildBranchReference.Unwrap()
			if childBranchReference == nil {
				currentCachedBranch.Release()
				cachedChildBranchReferences.Release()
				err = xerrors.Errorf("failed to load ChildBranch reference: %w", cerrors.ErrFatal)
				return
			}

			// queue child Branch for propagation
			branchStack.PushBack(childBranchReference.ChildBranchID())
		}
		cachedChildBranchReferences.Release()

		// release current CachedBranch
		currentCachedBranch.Release()
	}

	return
}

// updateInclusionState is an internal utility function that updates the InclusionState of the given Branch (and its
// future cone) if it fulfills the conditions.
func (b *BranchDAG) updateInclusionState(branchID BranchID, inclusionState InclusionState) (err error) {
	// initialize stack for iteration
	branchStack := list.New()
	branchStack.PushBack(branchID)

	// iterate through stack
ProcessStack:
	for branchStack.Len() >= 1 {
		// retrieve first element from the stack
		currentStackElement := branchStack.Front()
		branchStack.Remove(currentStackElement)

		// load Branch
		currentCachedBranch := b.Branch(currentStackElement.Value.(BranchID))

		// unwrap current CachedBranch
		currentBranch := currentCachedBranch.Unwrap()
		if currentBranch == nil {
			currentCachedBranch.Release()
			err = xerrors.Errorf("failed to load Branch with %s: %w", currentCachedBranch.ID(), cerrors.ErrFatal)
			return
		}

		// execute case dependent logic
		switch inclusionState {
		case Confirmed:
			// abort if the current Branch is not liked or not finalized
			if !currentBranch.Liked() || !currentBranch.Finalized() {
				currentCachedBranch.Release()
				continue
			}

			// abort if any parent Branch is not confirmed
			for parentBranchID := range currentBranch.Parents() {
				// load parent Branch
				cachedParentBranch := b.Branch(parentBranchID)

				// unwrap parent Branch
				parentBranch := cachedParentBranch.Unwrap()
				if parentBranch == nil {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					err = xerrors.Errorf("failed to load parent Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
					return
				}

				// abort if parent Branch is not confirmed
				if parentBranch.InclusionState() != Confirmed {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					continue ProcessStack
				}

				// release parent CachedBranch
				cachedParentBranch.Release()
			}

			// abort if the Branch is already confirmed
			if !currentBranch.SetInclusionState(Confirmed) {
				currentCachedBranch.Release()
				continue
			}

			// trigger event
			b.Events.BranchConfirmed.Trigger(NewBranchDAGEvent(currentCachedBranch))
		case Rejected:
			// abort if the current Branch is not confirmed already
			if !currentBranch.SetInclusionState(Rejected) {
				currentCachedBranch.Release()
				continue
			}

			// trigger event
			b.Events.BranchRejected.Trigger(NewBranchDAGEvent(currentCachedBranch))
		case Pending:
			// abort if the current Branch is not confirmed already
			if !currentBranch.SetInclusionState(Pending) {
				currentCachedBranch.Release()
				continue
			}

			// trigger event
			b.Events.BranchPending.Trigger(NewBranchDAGEvent(currentCachedBranch))
		}

		// iterate through ChildBranch references and queue found Branches for propagation
		cachedChildBranchReferences := b.ChildBranches(currentBranch.ID())
		for _, cachedChildBranchReference := range cachedChildBranchReferences {
			// unwrap ChildBranch reference
			childBranchReference := cachedChildBranchReference.Unwrap()
			if childBranchReference == nil {
				currentCachedBranch.Release()
				cachedChildBranchReferences.Release()
				err = xerrors.Errorf("failed to load ChildBranch reference: %w", cerrors.ErrFatal)
				return
			}

			// queue child Branch for propagation
			branchStack.PushBack(childBranchReference.ChildBranchID())
		}
		cachedChildBranchReferences.Release()

		// release current CachedBranch
		currentCachedBranch.Release()
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchDAGEvents //////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAGEvents is a container for all of the BranchDAG related events.
type BranchDAGEvents struct {
	// BranchLiked gets triggered whenever a Branch becomes liked that was not liked before.
	BranchLiked *events.Event

	// BranchDisliked gets triggered whenever a Branch becomes disliked that was liked before.
	BranchDisliked *events.Event

	// BranchMonotonicallyLiked gets triggered whenever a Branch becomes monotonically liked that was not monotonically
	// liked before.
	BranchMonotonicallyLiked *events.Event

	// BranchMonotonicallyDisliked gets triggered whenever a Branch becomes monotonically disliked that was
	// monotonically liked before.
	BranchMonotonicallyDisliked *events.Event

	// BranchFinalized gets triggered when a decision on a Branch is finalized and there will be no further state
	// changes regarding its liked state.
	BranchFinalized *events.Event

	// BranchUnfinalized gets triggered when a previously finalized Branch is marked as not finalized again (i.e. during
	// a reorg).
	BranchUnfinalized *events.Event

	// BranchConfirmed gets triggered whenever a Branch becomes confirmed that was not confirmed before.
	BranchConfirmed *events.Event

	// BranchRejected gets triggered whenever a Branch becomes rejected that was not rejected before.
	BranchRejected *events.Event

	// BranchPending gets triggered whenever a Branch becomes pending that was not pending before (i.e. during a reorg).
	BranchPending *events.Event
}

// NewBranchDAGEvents creates a container for all of the BranchDAG related events.
func NewBranchDAGEvents() *BranchDAGEvents {
	return &BranchDAGEvents{
		BranchLiked:                 events.NewEvent(branchEventCaller),
		BranchDisliked:              events.NewEvent(branchEventCaller),
		BranchMonotonicallyLiked:    events.NewEvent(branchEventCaller),
		BranchMonotonicallyDisliked: events.NewEvent(branchEventCaller),
		BranchFinalized:             events.NewEvent(branchEventCaller),
		BranchUnfinalized:           events.NewEvent(branchEventCaller),
		BranchConfirmed:             events.NewEvent(branchEventCaller),
		BranchRejected:              events.NewEvent(branchEventCaller),
		BranchPending:               events.NewEvent(branchEventCaller),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchDAGEvent ///////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAGEvent represents an event object that contains a Branch and that is triggered by the BranchDAG.
type BranchDAGEvent struct {
	Branch *CachedBranch
}

// NewBranchDAGEvent creates a new BranchDAGEvent from the given CachedBranch.
func NewBranchDAGEvent(branch *CachedBranch) (newBranchEvent *BranchDAGEvent) {
	return &BranchDAGEvent{Branch: branch}
}

// Retain marks all of the CachedObjects in the event to be retained for future use.
func (b *BranchDAGEvent) Retain() *BranchDAGEvent {
	return &BranchDAGEvent{
		Branch: b.Branch.Retain(),
	}
}

// Release marks all of the CachedObjects in the event as not used anymore.
func (b *BranchDAGEvent) Release() *BranchDAGEvent {
	b.Branch.Release()

	return b
}

// branchEventCaller is an internal utility function that type casts the generic parameters of the event handler to
// their specific type. It automatically retains all the contained CachedObjects for their use in the registered
// callback.
func branchEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(branch *BranchDAGEvent))(params[0].(*BranchDAGEvent).Retain())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
