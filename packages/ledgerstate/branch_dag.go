package ledgerstate

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region BranchDAG ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAGCacheSize defines how many elements are stored in the internal LRUCaches.
const BranchDAGCacheSize = 1024

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions of the ledger state that exist in the tangle.
type BranchDAG struct {
	branchStorage         *objectstorage.ObjectStorage
	childBranchStorage    *objectstorage.ObjectStorage
	conflictStorage       *objectstorage.ObjectStorage
	conflictMemberStorage *objectstorage.ObjectStorage
	shutdownOnce          sync.Once
	Events                *BranchDAGEvents

	inclusionStateMutex sync.RWMutex
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(ledgerstate *Ledgerstate) (newBranchDAG *BranchDAG) {
	options := buildObjectStorageOptions(ledgerstate.Options.CacheTimeProvider)
	osFactory := objectstorage.NewFactory(ledgerstate.Options.Store, database.PrefixLedgerState)
	newBranchDAG = &BranchDAG{
		branchStorage:         osFactory.New(PrefixBranchStorage, BranchFromObjectStorage, options.branchStorageOptions...),
		childBranchStorage:    osFactory.New(PrefixChildBranchStorage, ChildBranchFromObjectStorage, options.childBranchStorageOptions...),
		conflictStorage:       osFactory.New(PrefixConflictStorage, ConflictFromObjectStorage, options.conflictStorageOptions...),
		conflictMemberStorage: osFactory.New(PrefixConflictMemberStorage, ConflictMemberFromObjectStorage, options.conflictMemberStorageOptions...),
		Events: &BranchDAGEvents{
			BranchCreated:        events.NewEvent(BranchIDEventHandler),
			BranchParentsUpdated: events.NewEvent(branchParentUpdateEventCaller),
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

	resolvedParentBranchIDs, err := b.ResolveConflictBranchIDs(parentBranchIDs)
	if err != nil {
		err = errors.Errorf("failed to resolve conflict BranchIDs: %w", err)
		return
	}

	// create or load the branch
	cachedConflictBranch = b.Branch(branchID, func() Branch {
		conflictBranch := NewConflictBranch(branchID, resolvedParentBranchIDs, conflictIDs)

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
		for resolvedParentBranchID := range resolvedParentBranchIDs {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(resolvedParentBranchID, branchID, ConflictBranchType)); stored {
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

	b.inclusionStateMutex.RUnlock()

	if newBranchCreated {
		b.Events.BranchCreated.Trigger(branchID)
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

	b.Events.BranchParentsUpdated.Trigger(&BranchParentUpdate{conflictBranchID, newParentBranchIDs})

	return
}

// ResolveConflictBranchIDs returns the BranchIDs of the ConflictBranches that the given Branches represent by resolving
// AggregatedBranches to their corresponding ConflictBranches.
func (b *BranchDAG) ResolveConflictBranchIDs(branchIDs BranchIDs) (conflictBranchIDs BranchIDs, err error) {
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
				result.Add(branch.ID())
			case AggregatedBranchType:
				for parentBranchID := range branch.Parents() {
					result.Add(parentBranchID)
				}
			}
		}) {
			return nil, errors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
		}
	}

	return result, nil

}

// ResolvePendingConflictBranchIDs returns the BranchIDs of the pending and rejected ConflictBranches that are
// addressed by the given BranchIDs.
func (b *BranchDAG) ResolvePendingConflictBranchIDs(branchIDs BranchIDs) (conflictBranchIDs BranchIDs, err error) {
	branchWalker := walker.New()
	for branchID := range branchIDs {
		branchWalker.Push(branchID)
	}

	result := make(BranchIDs)
	for branchWalker.HasNext() {
		currentBranchID := branchWalker.Next().(BranchID)

		if !b.Branch(currentBranchID).Consume(func(branch Branch) {
			switch typeCastedBranch := branch.(type) {
			case *ConflictBranch:
				if typeCastedBranch.InclusionState() == Confirmed {
					return
				}

				result.Add(branch.ID())
			case *AggregatedBranch:
				for parentBranchID := range branch.Parents() {
					branchWalker.Push(parentBranchID)
				}
			}
		}) {
			return nil, errors.Errorf("failed to load Branch with %s: %w", currentBranchID, cerrors.ErrFatal)
		}
	}

	if len(result) == 0 {
		return NewBranchIDs(MasterBranchID), nil
	}

	return result, nil
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
func (b *BranchDAG) ForEachConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID) bool) {
	resolvedConflictBranchIDs, err := b.ResolveConflictBranchIDs(NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	abort := false
	for conflictBranchID := range resolvedConflictBranchIDs {
		b.Branch(conflictBranchID).Consume(func(branch Branch) {
			for conflictID := range branch.(*ConflictBranch).Conflicts() {
				b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
					if abort || conflictMember.BranchID() == conflictBranchID {
						return
					}

					abort = !callback(conflictMember.BranchID())
				})
				if abort {
					return
				}
			}
		})
		if abort {
			return
		}
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
		(&CachedBranch{CachedObject: cachedMasterBranch}).ConsumeConflictBranch(func(conflictBranch *ConflictBranch) {
			conflictBranch.setInclusionState(Confirmed)
		})
	}
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

// AggregateConflictBranchesID returns the aggregated BranchID that represents the given BranchIDs.
func (b *BranchDAG) AggregateConflictBranchesID(parentBranchIDs BranchIDs) (branchID BranchID) {
	branchIDs := NewBranchIDs()
	for branchID := range parentBranchIDs {
		b.Branch(branchID).ConsumeConflictBranch(func(conflictBranch *ConflictBranch) {
			if conflictBranch.InclusionState() == Confirmed {
				return
			}
			branchIDs.Add(branchID)
		})
	}

	if len(branchIDs) == 0 {
		return MasterBranchID
	}

	if len(branchIDs) == 1 {
		for firstBranchID := range branchIDs {
			return firstBranchID
		}
	}

	aggregatedBranch := NewAggregatedBranch(branchIDs)
	(&CachedBranch{CachedObject: b.branchStorage.ComputeIfAbsent(aggregatedBranch.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		aggregatedBranch.Persist()
		aggregatedBranch.SetModified()

		return aggregatedBranch
	})}).Release()

	return aggregatedBranch.ID()
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

type BranchDAGEvents struct {
	// BranchCreated gets triggered when a new Branch is created.
	BranchCreated *events.Event

	// BranchParentsUpdated gets triggered whenever a Branch's parents are updated.
	BranchParentsUpdated *events.Event
}

// BranchParentUpdate contains the new branch parents of a branch.
type BranchParentUpdate struct {
	ID         BranchID
	NewParents BranchIDs
}

func branchParentUpdateEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(branchParents *BranchParentUpdate))(params[0].(*BranchParentUpdate))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
