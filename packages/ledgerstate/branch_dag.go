package ledgerstate

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
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

	inclusionStateMutex sync.RWMutex
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(ledgerstate *Ledgerstate) (newBranchDAG *BranchDAG) {
	options := buildObjectStorageOptions(ledgerstate.Options.CacheTimeProvider)
	osFactory := objectstorage.NewFactory(ledgerstate.Options.Store, database.PrefixLedgerState)
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

// region CORE API /////////////////////////////////////////////////////////////////////////////////////////////////////

// CreateConflictBranch retrieves the ConflictBranch that corresponds to the given details. It automatically creates and
// updates the ConflictBranch according to the new details if necessary.
func (b *BranchDAG) CreateConflictBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (cachedConflictBranch *CachedBranch, newBranchCreated bool, err error) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	normalizedParentBranchIDs, _, err := b.normalizeBranches(parentBranchIDs)
	if err != nil {
		return nil, false, errors.Errorf("failed to normalize parent Branches: %w", err)
	}

	// create or load the branch
	cachedConflictBranch = b.Branch(branchID, func() Branch {
		conflictBranch := NewConflictBranch(branchID, normalizedParentBranchIDs, conflictIDs)
		conflictBranch.Persist()
		conflictBranch.SetModified()

		newBranchCreated = true

		return conflictBranch
	}).Retain()

	cachedConflictBranch.ConsumeConflictBranch(func(conflictBranch *ConflictBranch) {
		// If the branch existed already we simply update its conflict members.
		//
		// An existing Branch can only become a new member of a conflict set if that conflict set was newly created in which
		// case none of the members of that set can either be Confirmed or Rejected. This means that our InclusionState does
		// not change, and we don't need to update and propagate it.
		if !newBranchCreated {
			for conflictID := range conflictIDs {
				if conflictBranch.AddConflict(conflictID) {
					b.registerConflictMember(conflictID, branchID)
				}
			}
			return
		}

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

		if b.anyParentRejected(conflictBranch) || b.anyConflictMemberConfirmed(conflictBranch) {
			conflictBranch.setInclusionState(Rejected)
		}
	})

	if newBranchCreated {
		b.Events.BranchCreated.Trigger(branchID)
		return
	}

	return
}

// UpdateConflictBranchParents changes the parents of a ConflictBranch (also updating the references of the
// ChildBranches).
func (b *BranchDAG) UpdateConflictBranchParents(conflictBranchID BranchID, newParentBranchIDs BranchIDs) (err error) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

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

	newParentBranchIDs, _, err = b.normalizeBranches(newParentBranchIDs)
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

	return conflictBranchIDs, err
}

// AggregateBranches retrieves the AggregatedBranch that corresponds to the given BranchIDs. It automatically creates
// the AggregatedBranch if it didn't exist, yet.
func (b *BranchDAG) AggregateBranches(branchIDs BranchIDs) (cachedAggregatedBranch *CachedBranch, newBranchCreated bool, err error) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	normalizedBranchIDs, _, err := b.normalizeBranches(branchIDs)
	if err != nil {
		err = errors.Errorf("failed to normalize Branches: %w", err)
		return
	}

	return b.aggregateNormalizedBranches(normalizedBranchIDs)
}

// SetBranchConfirmed sets the InclusionState of the given Branch to be Confirmed.
func (b *BranchDAG) SetBranchConfirmed(branchID BranchID) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	conflictBranchIDs, err := b.ResolveConflictBranchIDs(NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	confirmationWalker := walker.New()
	for conflictBranchID := range conflictBranchIDs {
		confirmationWalker.Push(conflictBranchID)
	}
	rejectedWalker := walker.New()

	for confirmationWalker.HasNext() {
		currentBranchID := confirmationWalker.Next().(BranchID)

		b.Branch(currentBranchID).ConsumeConflictBranch(func(branch *ConflictBranch) {
			if modified = branch.setInclusionState(Confirmed); !modified {
				return
			}

			for parentBranchID := range branch.Parents() {
				confirmationWalker.Push(parentBranchID)
			}

			for conflictID := range branch.Conflicts() {
				b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
					if conflictMember.BranchID() != currentBranchID {
						rejectedWalker.Push(conflictMember.BranchID())
					}
				})
			}
		})
	}

	for rejectedWalker.HasNext() {
		b.Branch(rejectedWalker.Next().(BranchID)).ConsumeConflictBranch(func(branch *ConflictBranch) {
			if modified = branch.setInclusionState(Rejected); !modified {
				return
			}

			b.ChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
				if childBranch.ChildBranchType() == ConflictBranchType {
					rejectedWalker.Push(childBranch.ChildBranchID())
				}
			})
		})
	}

	return modified
}

// InclusionState returns the InclusionState of the given Branch.
func (b *BranchDAG) InclusionState(branchID BranchID) (inclusionState InclusionState) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	b.Branch(branchID).Consume(func(branch Branch) {
		if conflictBranch, isConflictBranch := branch.(*ConflictBranch); isConflictBranch {
			inclusionState = conflictBranch.InclusionState()
			return
		}

		isParentRejected := false
		isParentPending := false
		for parentBranchID := range branch.Parents() {
			b.Branch(parentBranchID).ConsumeConflictBranch(func(parentBranch *ConflictBranch) {
				parentInclusionState := parentBranch.InclusionState()

				isParentRejected = parentInclusionState == Rejected
				isParentPending = isParentPending || parentInclusionState == Pending
			})

			if isParentRejected {
				inclusionState = Rejected
				return
			}
		}

		if isParentPending {
			inclusionState = Pending
			return
		}

		inclusionState = Confirmed
	})

	return inclusionState
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region STORAGE API //////////////////////////////////////////////////////////////////////////////////////////////////

// Branch retrieves the Branch with the given BranchID from the object storage.
func (b *BranchDAG) Branch(branchID BranchID, computeIfAbsentCallback ...func() Branch) (cachedBranch *CachedBranch) {
	if len(computeIfAbsentCallback) >= 1 {
		return &CachedBranch{b.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) objectstorage.StorableObject {
			return computeIfAbsentCallback[0]()
		})}
	}

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

// ForEachBranch iterates over all the branches and executes consumer.
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

// ForEachConnectedConflictingBranchID executes the callback for each ConflictBranch that is connected through a chain
// of intersecting ConflictSets.
func (b *BranchDAG) ForEachConnectedConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID)) {
	resolvedConflictBranchIDs, err := b.ResolveConflictBranchIDs(NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	traversedBranches := set.New()
	conflictSetsWalker := walker.New()

	processBranchAndQueueConflictSets := func(conflictBranchID BranchID) {
		if !traversedBranches.Add(conflictBranchID) {
			return
		}

		b.Branch(conflictBranchID).ConsumeConflictBranch(func(conflictBranch *ConflictBranch) {
			for conflictID := range conflictBranch.Conflicts() {
				conflictSetsWalker.Push(conflictID)
			}
		})
	}

	for conflictBranchID := range resolvedConflictBranchIDs {
		processBranchAndQueueConflictSets(conflictBranchID)
	}

	for conflictSetsWalker.HasNext() {
		b.ConflictMembers(conflictSetsWalker.Next().(ConflictID)).Consume(func(conflictMember *ConflictMember) {
			processBranchAndQueueConflictSets(conflictMember.BranchID())
		})
	}

	traversedBranches.ForEach(func(element interface{}) {
		callback(element.(BranchID))
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PRIVATE UTILITY FUNCTIONS ////////////////////////////////////////////////////////////////////////////////////

// init is an internal utility function that initializes the BranchDAG by creating the root of the DAG (MasterBranch).
func (b *BranchDAG) init() {
	cachedMasterBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(MasterBranchID, nil, nil))
	if stored {
		cachedMasterBranch.Release()
	}

	cachedInvalidBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(InvalidBranchID, nil, nil))
	if stored {
		cachedInvalidBranch.Release()
	}
}

// normalizeBranches is an internal utility function that takes a list of BranchIDs and returns the BranchIDS of the
// most special ConflictBranches that the given BranchIDs represent. It returns an error if the Branches are conflicting
// or any other unforeseen error occurred.
func (b *BranchDAG) normalizeBranches(branchIDs BranchIDs) (branches BranchIDs, lazyBooked bool, err error) {
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
		seenBranches := set.New()
		seenConflictSets := make(map[ConflictID]BranchID)
		parentsWalker := walker.New()

		// create normalized branch candidates (check their conflicts and queue parent checks)
		branches = NewBranchIDs()
		for conflictBranchID := range conflictBranches {
			// check branch and queue parents
			if !b.Branch(conflictBranchID).ConsumeConflictBranch(func(branch *ConflictBranch) {
				switch branch.InclusionState() {
				case Rejected:
					lazyBooked = true
					fallthrough
				case Pending:
					// add branch to the candidates of normalized branches
					branches.Add(conflictBranchID)
				}

				if err = b.queueParentsIfConflictSetUnseen(branch, seenBranches, seenConflictSets, parentsWalker); err != nil {
					err = errors.Errorf("failed to check conflicts and queue parents: %w", err)
					return
				}
			}) {
				return errors.Errorf("failed to load Branch with %s: %w", conflictBranchID, cerrors.ErrFatal)
			}

			// abort if we faced an error
			if err != nil {
				return err
			}
		}

		// remove ancestors from the candidates
		for parentsWalker.HasNext() {
			// retrieve parent branch ID from stack
			parentBranchID := parentsWalker.Next().(BranchID)

			// remove ancestor from normalized candidates
			delete(branches, parentBranchID)

			// check branch, queue parents and abort if we faced an error
			if !b.Branch(parentBranchID).ConsumeConflictBranch(func(conflictBranch *ConflictBranch) {
				err = b.queueParentsIfConflictSetUnseen(conflictBranch, seenBranches, seenConflictSets, parentsWalker)
			}) {
				return errors.Errorf("failed to load Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
			}

			// abort if we faced an error
			if err != nil {
				return err
			}
		}

		return branches
	}).(type) {
	case error:
		err = typeCastedResult
	case BranchIDs:
		branches = typeCastedResult
	}

	if len(branches) == 0 {
		branches = NewBranchIDs(MasterBranchID)
	}

	return branches, lazyBooked, err
}

func (b *BranchDAG) queueParentsIfConflictSetUnseen(branch *ConflictBranch, traversedBranches set.Set, seenConflictSets map[ConflictID]BranchID, parentWalker *walker.Walker) (err error) {
	// abort if branch was traversed already
	if !traversedBranches.Add(branch.ID()) {
		return
	}

	// return error if conflict set was seen twice
	for conflictSetID := range branch.Conflicts() {
		if conflictingBranch, exists := seenConflictSets[conflictSetID]; exists {
			return errors.Errorf("%s conflicts with %s in %s: %w", conflictingBranch, branch.ID(), conflictSetID, ErrInvalidStateTransition)
		}
		seenConflictSets[conflictSetID] = branch.ID()
	}

	if branch.InclusionState() == Confirmed {
		return
	}

	// queue parents to be checked when traversing ancestors
	for parentBranchID := range branch.Parents() {
		parentWalker.Push(parentBranchID)
	}

	return nil
}

func (b *BranchDAG) anyParentRejected(conflictBranch *ConflictBranch) (parentRejected bool) {
	for parentBranchID := range conflictBranch.Parents() {
		b.Branch(parentBranchID).ConsumeConflictBranch(func(parentBranch *ConflictBranch) {
			if parentRejected = parentBranch.InclusionState() == Rejected; parentRejected {
				return
			}
		})

		if parentRejected {
			return
		}
	}

	return
}

// anyConflictMemberConfirmed makes a ConflictBranch rejected if any of its conflicting Branches is
// Confirmed.
func (b *BranchDAG) anyConflictMemberConfirmed(branch *ConflictBranch) (conflictMemberConfirmed bool) {
	for conflictID := range branch.Conflicts() {
		b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			if conflictMemberConfirmed || conflictMember.BranchID() == branch.ID() {
				return
			}

			b.Branch(conflictMember.BranchID()).ConsumeConflictBranch(func(conflictingBranch *ConflictBranch) {
				conflictMemberConfirmed = conflictingBranch.InclusionState() == Confirmed
			})
		})

		if conflictMemberConfirmed {
			return
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchDAGEvents //////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAGEvents is a container for all BranchDAG related events.
type BranchDAGEvents struct {
	// BranchCreated gets triggered when a new Branch is created.
	BranchCreated *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
