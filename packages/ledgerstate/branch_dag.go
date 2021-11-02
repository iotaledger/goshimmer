package ledgerstate

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/stack"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lru_cache"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region BranchDAG ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAGCacheSize defines how many elements are stored in the internal LRUCaches.
const BranchDAGCacheSize = 1024

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions of the ledger state that exist in the tangle.
type BranchDAG struct {
	branchStorage          *objectstorage.ObjectStorage
	childBranchStorage     *objectstorage.ObjectStorage
	conflictStorage        *objectstorage.ObjectStorage
	conflictMemberStorage  *objectstorage.ObjectStorage
	shutdownOnce           sync.Once
	normalizedBranchCache  *lru_cache.LRUCache
	conflictBranchIDsCache *lru_cache.LRUCache
	Events                 *BranchDAGEvents
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(store kvstore.KVStore, cacheProvider *database.CacheTimeProvider) (newBranchDAG *BranchDAG) {
	options := buildObjectStorageOptions(cacheProvider)
	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)
	newBranchDAG = &BranchDAG{
		branchStorage:          osFactory.New(PrefixBranchStorage, BranchFromObjectStorage, options.branchStorageOptions...),
		childBranchStorage:     osFactory.New(PrefixChildBranchStorage, ChildBranchFromObjectStorage, options.childBranchStorageOptions...),
		conflictStorage:        osFactory.New(PrefixConflictStorage, ConflictFromObjectStorage, options.conflictStorageOptions...),
		conflictMemberStorage:  osFactory.New(PrefixConflictMemberStorage, ConflictMemberFromObjectStorage, options.conflictMemberStorageOptions...),
		normalizedBranchCache:  lru_cache.NewLRUCache(BranchDAGCacheSize),
		conflictBranchIDsCache: lru_cache.NewLRUCache(BranchDAGCacheSize),
		Events: &BranchDAGEvents{
			BranchCreated: events.NewEvent(BranchIDEventHandler),
		},
	}
	newBranchDAG.init()

	return
}

// CreateConflictBranch retrieves the ConflictBranch that corresponds to the given details. It automatically creates and
// updates the ConflictBranch according to the new details if necessary.
func (b *BranchDAG) CreateConflictBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (cachedConflictBranch *CachedBranch, newBranchCreated bool, err error) {
	normalizedParentBranchIDs, err := b.normalizeBranches(parentBranchIDs)
	if err != nil {
		err = errors.Errorf("failed to normalize parent Branches: %w", err)
		return
	}

	cachedConflictBranch, newBranchCreated, err = b.createConflictBranchFromNormalizedParentBranchIDs(branchID, normalizedParentBranchIDs, conflictIDs)
	if newBranchCreated {
		b.Events.BranchCreated.Trigger(branchID)
	}
	return
}

// UpdateConflictBranchParents changes the parents of a ConflictBranch (also updating the references of the
// ChildBranches).
func (b *BranchDAG) UpdateConflictBranchParents(conflictBranchID BranchID, newParentBranchIDs BranchIDs) (err error) {
	cachedConflictBranch := b.Branch(conflictBranchID)
	defer cachedConflictBranch.Release()

	conflictBranch, err := cachedConflictBranch.UnwrapConflictBranch()
	if err != nil {
		err = errors.Errorf("failed to unwrap ConflictBranch: %w", err)
		return
	}
	if conflictBranch == nil {
		err = errors.Errorf("failed to unwrap ConflictBranch: %w", cerrors.ErrFatal)
		return
	}

	newParentBranchIDs, err = b.normalizeBranches(newParentBranchIDs)
	if err != nil {
		err = errors.Errorf("failed to normalize new parent BranchIDs: %w", err)
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
		err = errors.Errorf("failed to normalize Branches: %w", err)
		return
	}

	cachedAggregatedBranch, newBranchCreated, err = b.aggregateNormalizedBranches(normalizedBranchIDs)
	return
}

// MergeToMaster merges a confirmed Branch with the MasterBranch to clean up the BranchDAG. It reorganizes existing
// ChildBranches by adjusting their parents accordingly.
/*
func (b *BranchDAG) MergeToMaster(branchID BranchID) (movedBranches map[BranchID]BranchID, err error) {
	movedBranches = make(map[BranchID]BranchID)

	// load Branch
	cachedBranch := b.Branch(branchID)
	defer cachedBranch.Release()

	// unwrap ConflictBranch
	conflictBranch, err := cachedBranch.UnwrapConflictBranch()
	if err != nil {
		err = errors.Errorf("tried to merge non-ConflictBranch with %s to Master: %w", branchID, err)
		return
	} else if conflictBranch == nil {
		err = errors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
		return
	}

	// abort if the Branch is not High GoF
	if b.tangle.ConfirmationOracle.IsBranchConfirmed(conflictBranch) {
		err = errors.Errorf("tried to merge non-high-gof Branch with %s to Master: %w", branchID, cerrors.ErrFatal)
		return
	}

	// abort if the Branch is not at the bottom of the BranchDAG
	parentBranches := conflictBranch.Parents()
	if _, masterBranchIsParent := parentBranches[MasterBranchID]; len(parentBranches) != 1 || !masterBranchIsParent {
		err = errors.Errorf("tried to merge Branch with %s to Master that is not at the bottom of the BranchDAG: %w", branchID, err)
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
			err = errors.Errorf("failed to load ChildBranch reference: %w", cerrors.ErrFatal)
			return
		}

		switch childBranchReference.ChildBranchType() {
		case AggregatedBranchType:
			// load referenced ChildBranch
			cachedChildBranch := b.Branch(childBranchReference.ChildBranchID())
			childBranch, unwrapErr := cachedChildBranch.UnwrapAggregatedBranch()
			if unwrapErr != nil {
				cachedChildBranch.Release()
				err = errors.Errorf("failed to load AggregatedBranch with %s: %w", cachedChildBranch.ID(), unwrapErr)
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
					err = errors.Errorf("failed to retrieve AggregatedBranch: %w", newAggregatedBranchErr)
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
				err = errors.Errorf("failed to load ConflictBranch with %s: %w", cachedChildBranch.ID(), unwrapErr)
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
*/

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
	}, objectstorage.WithIteratorPrefix(branchID.Bytes()))

	return
}

// ForEachBranch iterates over all of the branches and executes consumer.
func (b *BranchDAG) ForEachBranch(consumer func(branch Branch)) {
	b.branchStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		(&CachedBranch{CachedObject: cachedObject}).Consume(func(branch Branch) {
			consumer(branch)
		})

		return true
	})
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
	}, objectstorage.WithIteratorPrefix(conflictID.Bytes()))

	return
}

// ResolveConflictBranchIDs returns the BranchIDs of the ConflictBranches that the given Branches represent by resolving
// AggregatedBranches to their corresponding ConflictBranches.
func (b *BranchDAG) ResolveConflictBranchIDs(branchIDs BranchIDs) (conflictBranchIDs BranchIDs, err error) {
	switch typeCastedResult := b.conflictBranchIDsCache.ComputeIfAbsent(NewAggregatedBranch(branchIDs).ID(), func() interface{} {
		// initialize return variable
		result := make(BranchIDs)

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
					result[branch.ID()] = types.Void
				case AggregatedBranchType:
					for parentBranchID := range branch.Parents() {
						result[parentBranchID] = types.Void
					}
				}
			}) {
				return errors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
			}
		}

		return result
	}).(type) {
	case error:
		err = typeCastedResult
	case BranchIDs:
		conflictBranchIDs = typeCastedResult
	}

	return
}

// ForEachConflictingBranchID executes the callback for each ConflictBranch that is conflicting with the Branch
// identified by the given BranchID.
func (b *BranchDAG) ForEachConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID)) {
	resolvedConflictBranchIDs, err := b.ResolveConflictBranchIDs(NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	for conflictBranchID := range resolvedConflictBranchIDs {
		b.Branch(conflictBranchID).Consume(func(branch Branch) {
			for conflictID := range branch.(*ConflictBranch).Conflicts() {
				b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
					if conflictMember.BranchID() == conflictBranchID {
						return
					}

					callback(conflictMember.BranchID())
				})
			}
		})
	}
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
			err = errors.Errorf("failed to prune the object storage (%v): %w", err, cerrors.ErrFatal)
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
		b.normalizedBranchCache = nil
		b.conflictBranchIDsCache = nil
	})
}

// init is an internal utility function that initializes the BranchDAG by creating the root of the DAG (MasterBranch).
func (b *BranchDAG) init() {
	cachedMasterBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(MasterBranchID, nil, nil))
	if stored {
		cachedMasterBranch.Release()
	}

	cachedRejectedBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(InvalidBranchID, nil, nil))
	if stored {
		cachedRejectedBranch.Release()
	}

	cachedLazyBookedConflictsBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(LazyBookedConflictsBranchID, nil, nil))
	if stored {
		cachedLazyBookedConflictsBranch.Release()
	}
}

// normalizeBranches is an internal utility function that takes a list of BranchIDs and returns the BranchIDS of the
// most special ConflictBranches that the given BranchIDs represent. It returns an error if the Branches are conflicting
// or any other unforeseen error occurred.
func (b *BranchDAG) normalizeBranches(branchIDs BranchIDs) (normalizedBranches BranchIDs, err error) {
	switch typeCastedResult := b.normalizedBranchCache.ComputeIfAbsent(NewAggregatedBranch(branchIDs).ID(), func() interface{} {
		// retrieve conflict branches and abort if we faced an error
		conflictBranches, conflictBranchesErr := b.ResolveConflictBranchIDs(branchIDs)
		if conflictBranchesErr != nil {
			return errors.Errorf("failed to resolve ConflictBranchIDs: %w", conflictBranchesErr)
		}

		// return if we are done
		if len(conflictBranches) == 1 {
			return conflictBranches
		}

		// return the master branch if the list of conflict branches is empty
		if len(conflictBranches) == 0 {
			return BranchIDs{MasterBranchID: types.Void}
		}

		// introduce iteration variables
		traversedBranches := set.New()
		seenConflictSets := make(map[ConflictID]BranchID)
		parentsToCheck := stack.New()

		// checks if branches are conflicting and queues parents to be checked
		checkConflictsAndQueueParents := func(currentBranch Branch) {
			currentConflictBranch, typeCastOK := currentBranch.(*ConflictBranch)
			if !typeCastOK {
				err = errors.Errorf("failed to type cast Branch with %s to ConflictBranch: %w", currentBranch.ID(), cerrors.ErrFatal)
				return
			}

			// abort if branch was traversed already
			if !traversedBranches.Add(currentConflictBranch.ID()) {
				return
			}

			// return error if conflict set was seen twice
			for conflictSetID := range currentConflictBranch.Conflicts() {
				if conflictingBranch, exists := seenConflictSets[conflictSetID]; exists {
					err = errors.Errorf("%s conflicts with %s in %s: %w", conflictingBranch, currentConflictBranch.ID(), conflictSetID, ErrInvalidStateTransition)
					return
				}
				seenConflictSets[conflictSetID] = currentConflictBranch.ID()
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
				return errors.Errorf("failed to load Branch with %s: %w", conflictBranchID, cerrors.ErrFatal)
			}

			// abort if we faced an error
			if err != nil {
				return err
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
				return errors.Errorf("failed to load Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
			}

			// abort if we faced an error
			if err != nil {
				return err
			}
		}

		return normalizedBranches
	}).(type) {
	case error:
		err = typeCastedResult
	case BranchIDs:
		normalizedBranches = typeCastedResult
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
		err = errors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
		return
	}

	// type cast to ConflictBranch
	conflictBranch, typeCastOK := branch.(*ConflictBranch)
	if !typeCastOK {
		err = errors.Errorf("failed to type cast Branch with %s to ConflictBranch: %w", branchID, cerrors.ErrFatal)
		return
	}

	// register references
	switch {
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
func (b *BranchDAG) aggregateNormalizedBranches(parentBranchIDs BranchIDs) (cachedAggregatedBranch *CachedBranch, newBranchCreated bool, err error) {
	if len(parentBranchIDs) == 1 {
		for firstBranchID := range parentBranchIDs {
			cachedAggregatedBranch = b.Branch(firstBranchID)
			return
		}
	}

	aggregatedBranch := NewAggregatedBranch(parentBranchIDs)
	cachedAggregatedBranch = &CachedBranch{CachedObject: b.branchStorage.ComputeIfAbsent(aggregatedBranch.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		newBranchCreated = true

		aggregatedBranch.Persist()
		aggregatedBranch.SetModified()

		return aggregatedBranch
	})}

	if newBranchCreated {
		for parentBranchID := range parentBranchIDs {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, cachedAggregatedBranch.ID(), AggregatedBranchType)); stored {
				cachedChildBranch.Release()
			}
		}
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

// BranchDAGEvents is a container for all BranchDAG related events.
type BranchDAGEvents struct {
	// BranchCreated gets triggered when a new Branch is created.
	BranchCreated *events.Event
}
