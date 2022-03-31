package branchdag

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/database"
)

// block of default cache time.
const (
	branchCacheTime   = 60 * time.Second
	conflictCacheTime = 60 * time.Second
	consumerCacheTime = 10 * time.Second
)

const (
	// PrefixBranchStorage defines the storage prefix for the Branch object storage.
	PrefixBranchStorage byte = iota

	// PrefixChildBranchStorage defines the storage prefix for the ChildBranch object storage.
	PrefixChildBranchStorage

	// PrefixConflictStorage defines the storage prefix for the Conflict object storage.
	PrefixConflictStorage

	// PrefixConflictMemberStorage defines the storage prefix for the ConflictMember object storage.
	PrefixConflictMemberStorage

	// PrefixTransactionStorage defines the storage prefix for the Transaction object storage.
	PrefixTransactionStorage

	// PrefixTransactionMetadataStorage defines the storage prefix for the TransactionMetadata object storage.
	PrefixTransactionMetadataStorage

	// PrefixOutputStorage defines the storage prefix for the Output object storage.
	PrefixOutputStorage

	// PrefixOutputMetadataStorage defines the storage prefix for the OutputMetadata object storage.
	PrefixOutputMetadataStorage

	// PrefixConsumerStorage defines the storage prefix for the Consumer object storage.
	PrefixConsumerStorage

	// PrefixAddressOutputMappingStorage defines the storage prefix for the AddressOutputMapping object storage.
	PrefixAddressOutputMappingStorage
)

// region BranchDAG ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions of the ledger state that exist in the tangle.
type BranchDAG struct {
	Events *BranchDAGEvents

	branchStorage         *objectstorage.ObjectStorage[*Branch]
	childBranchStorage    *objectstorage.ObjectStorage[*ChildBranch]
	conflictStorage       *objectstorage.ObjectStorage[*Conflict]
	conflictMemberStorage *objectstorage.ObjectStorage[*ConflictMember]
	shutdownOnce          sync.Once
	inclusionStateMutex   sync.RWMutex
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(store kvstore.KVStore, cacheTimeProvider *database.CacheTimeProvider) (newBranchDAG *BranchDAG) {
	newBranchDAG = &BranchDAG{
		branchStorage: objectstorage.New[*Branch](
			store.WithRealm([]byte{database.PrefixLedger, PrefixBranchStorage}),
			cacheTimeProvider.CacheTime(branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		childBranchStorage: objectstorage.New[*ChildBranch](
			store.WithRealm([]byte{database.PrefixLedger, PrefixChildBranchStorage}),
			ChildBranchKeyPartition,
			cacheTimeProvider.CacheTime(branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		conflictStorage: objectstorage.New[*Conflict](
			store.WithRealm([]byte{database.PrefixLedger, PrefixConflictStorage}),
			cacheTimeProvider.CacheTime(consumerCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		conflictMemberStorage: objectstorage.New[*ConflictMember](
			store.WithRealm([]byte{database.PrefixLedger, PrefixConflictMemberStorage}),
			ConflictMemberKeyPartition,
			cacheTimeProvider.CacheTime(conflictCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		Events: &BranchDAGEvents{
			BranchCreated: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(BranchID))(params[0].(BranchID))
			}),
			BranchParentsUpdated: events.NewEvent(branchParentUpdateEventCaller),
		},
	}
	newBranchDAG.init()

	return
}

// region CORE API /////////////////////////////////////////////////////////////////////////////////////////////////////

// CreateBranch retrieves the Branch that corresponds to the given details. It automatically creates and
// updates the Branch according to the new details if necessary.
func (b *BranchDAG) CreateBranch(branchID *BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (created bool) {
	b.inclusionStateMutex.RLock()

	// create or load the branch
	b.Branch(branchID, func() *Branch {
		branch := NewBranch(branchID, parentBranchIDs, conflictIDs)

		created = true

		return branch
	}).Consume(func(branch *Branch) {
		// If the branch existed already we simply update its conflict members.
		//
		// An existing Branch can only become a new member of a conflict set if that conflict set was newly created in which
		// case none of the members of that set can either be Confirmed or Rejected. This means that our InclusionState does
		// not change, and we don't need to update and propagate it.
		if !created {
			_ = conflictIDs.ForEach(func(conflictID ConflictID) (err error) {
				if branch.AddConflict(conflictID) {
					b.registerConflictMember(conflictID, branchID)
				}

				return nil
			})
			return
		}

		// store child references
		_ = parentBranchIDs.ForEach(func(parentBranchID *BranchID) (err error) {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID)); stored {
				cachedChildBranch.Release()
			}
			return nil
		})

		// store ConflictMember references
		_ = conflictIDs.ForEach(func(conflictID ConflictID) (err error) {
			b.registerConflictMember(conflictID, branchID)
			return nil
		})

		if b.anyParentRejected(branch) || b.anyConflictMemberConfirmed(branch) {
			branch.setInclusionState(Rejected)
		}
	})

	b.inclusionStateMutex.RUnlock()

	if created {
		b.Events.BranchCreated.Trigger(branchID)
	}

	return created
}

// UpdateParentsAfterFork changes the parents of a Branch (also updating the references of the ChildBranches).
func (b *BranchDAG) UpdateParentsAfterFork(branchID, newParentBranchID *BranchID, previousParents BranchIDs) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	b.Branch(branchID).Consume(func(branch *Branch) {
		parentBranchIDs := branch.Parents()
		if !parentBranchIDs.Add(newParentBranchID) {
			return
		}

		parentBranchIDs.DeleteAll(previousParents)

		if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(newParentBranchID, branchID)); stored {
			cachedChildBranch.Release()
		}

		if branch.SetParents(parentBranchIDs) {
			b.Events.BranchParentsUpdated.Trigger(&BranchParentUpdate{branchID, parentBranchIDs})
		}
	})
}

// RemoveConfirmedBranches returns the BranchIDs of the pending and rejected Branches that are
// addressed by the given BranchIDs.
func (b *BranchDAG) RemoveConfirmedBranches(branchIDs BranchIDs) (pendingBranchIDs BranchIDs) {
	pendingBranchIDs = NewBranchIDs()

	branchWalker := walker.New[*BranchID]().PushAll(branchIDs.Slice()...)
	for branchWalker.HasNext() {
		currentBranchID := branchWalker.Next()

		b.Branch(currentBranchID).Consume(func(branch *Branch) {
			if branch.InclusionState() == Confirmed {
				return
			}
			pendingBranchIDs.Add(branch.ID())
		})
	}

	if pendingBranchIDs.Size() == 0 {
		pendingBranchIDs = NewBranchIDs(&MasterBranchID)
	}

	return pendingBranchIDs
}

// SetBranchConfirmed sets the InclusionState of the given Branch to be Confirmed.
func (b *BranchDAG) SetBranchConfirmed(branchID *BranchID) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	confirmationWalker := walker.New[*BranchID]().Push(branchID)
	rejectedWalker := walker.New[*BranchID]()

	for confirmationWalker.HasNext() {
		currentBranchID := confirmationWalker.Next()

		b.Branch(currentBranchID).Consume(func(branch *Branch) {
			if modified = branch.setInclusionState(Confirmed); !modified {
				return
			}

			_ = branch.Parents().ForEach(func(branchID *BranchID) (err error) {
				confirmationWalker.Push(branchID)
				return nil
			})

			_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
				b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
					if *conflictMember.BranchID() != *currentBranchID {
						rejectedWalker.Push(conflictMember.BranchID())
					}
				})

				return nil
			})
		})
	}

	for rejectedWalker.HasNext() {
		b.Branch(rejectedWalker.Next()).Consume(func(branch *Branch) {
			if modified = branch.setInclusionState(Rejected); !modified {
				return
			}

			b.ChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
				rejectedWalker.Push(childBranch.ChildBranchID())
			})
		})
	}

	return modified
}

// InclusionState returns the InclusionState of the given BranchIDs.
func (b *BranchDAG) InclusionState(branchIDs BranchIDs) (inclusionState InclusionState) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	inclusionState = Confirmed
	_ = branchIDs.ForEach(func(branchID *BranchID) (err error) {
		switch b.inclusionState(branchID) {
		case Rejected:
			inclusionState = Rejected
			return errors.New("abort")
		case Pending:
			inclusionState = Pending
		}

		return nil
	})

	return inclusionState
}

// inclusionState returns the InclusionState of the given BranchID.
func (b *BranchDAG) inclusionState(branchID *BranchID) (inclusionState InclusionState) {
	if !b.Branch(branchID).Consume(func(branch *Branch) {
		inclusionState = branch.InclusionState()
	}) {
		panic(fmt.Sprintf("failed to load %s", branchID))
	}

	return inclusionState
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (b *BranchDAG) Prune() (err error) {
	for _, storagePrune := range []func() error{
		b.branchStorage.Prune,
		b.childBranchStorage.Prune,
		b.conflictStorage.Prune,
		b.conflictMemberStorage.Prune,
	} {
		if err = storagePrune(); err != nil {
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
func (b *BranchDAG) Branch(branchID *BranchID, computeIfAbsentCallback ...func() *Branch) (cachedBranch *objectstorage.CachedObject[*Branch]) {
	if len(computeIfAbsentCallback) >= 1 {
		return b.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *Branch {
			return computeIfAbsentCallback[0]()
		})
	}

	return b.branchStorage.Load(branchID.Bytes())
}

// ChildBranches loads the references to the ChildBranches of the given Branch from the object storage.
func (b *BranchDAG) ChildBranches(branchID *BranchID) (cachedChildBranches objectstorage.CachedObjects[*ChildBranch]) {
	cachedChildBranches = make(objectstorage.CachedObjects[*ChildBranch], 0)
	b.childBranchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ChildBranch]) bool {
		cachedChildBranches = append(cachedChildBranches, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(branchID.Bytes()))

	return
}

// ForEachBranch iterates over all the branches and executes consumer.
func (b *BranchDAG) ForEachBranch(consumer func(branch *Branch)) {
	b.branchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Branch]) bool {
		cachedObject.Consume(func(branch *Branch) {
			consumer(branch)
		})

		return true
	})
}

// Conflict loads a Conflict from the object storage.
func (b *BranchDAG) Conflict(conflictID ConflictID) *objectstorage.CachedObject[*Conflict] {
	return b.conflictStorage.Load(conflictID.Bytes())
}

// ConflictMembers loads the referenced ConflictMembers of a Conflict from the object storage.
func (b *BranchDAG) ConflictMembers(conflictID ConflictID) (cachedConflictMembers objectstorage.CachedObjects[*ConflictMember]) {
	cachedConflictMembers = make(objectstorage.CachedObjects[*ConflictMember], 0)
	b.conflictMemberStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ConflictMember]) bool {
		cachedConflictMembers = append(cachedConflictMembers, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(conflictID.Bytes()))

	return
}

// ForEachConflictingBranchID executes the callback for each Branch that is conflicting with the Branch
// identified by the given BranchID.
func (b *BranchDAG) ForEachConflictingBranchID(branchID *BranchID, callback func(conflictingBranchID *BranchID) bool) {
	abort := false
	b.Branch(branchID).Consume(func(branch *Branch) {
		_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
			b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
				if abort || conflictMember.BranchID() == branchID {
					return
				}

				abort = !callback(conflictMember.BranchID())
			})

			if abort {
				return errors.New("abort")
			}

			return nil
		})
	})
}

// ForEachConnectedConflictingBranchID executes the callback for each Branch that is connected through a chain
// of intersecting ConflictSets.
func (b *BranchDAG) ForEachConnectedConflictingBranchID(branchID *BranchID, callback func(conflictingBranchID *BranchID)) {
	traversedBranches := set.New[*BranchID]()
	conflictSetsWalker := walker.New[ConflictID]()

	processBranchAndQueueConflictSets := func(branchID *BranchID) {
		if !traversedBranches.Add(branchID) {
			return
		}

		b.Branch(branchID).Consume(func(branch *Branch) {
			_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
				conflictSetsWalker.Push(conflictID)
				return nil
			})
		})
	}

	processBranchAndQueueConflictSets(branchID)

	for conflictSetsWalker.HasNext() {
		b.ConflictMembers(conflictSetsWalker.Next()).Consume(func(conflictMember *ConflictMember) {
			processBranchAndQueueConflictSets(conflictMember.BranchID())
		})
	}

	traversedBranches.ForEach(func(element *BranchID) {
		callback(element)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PRIVATE UTILITY FUNCTIONS ////////////////////////////////////////////////////////////////////////////////////

// init is an internal utility function that initializes the BranchDAG by creating the root of the DAG (MasterBranch).
func (b *BranchDAG) init() {
	cachedMasterBranch, stored := b.branchStorage.StoreIfAbsent(NewBranch(&MasterBranchID, NewBranchIDs(), NewConflictIDs()))
	if stored {
		cachedMasterBranch.Consume(func(branch *Branch) {
			branch.setInclusionState(Confirmed)
		})
	}
}

func (b *BranchDAG) anyParentRejected(conflictBranch *Branch) (parentRejected bool) {
	_ = conflictBranch.Parents().ForEach(func(parentBranchID *BranchID) (err error) {
		b.Branch(parentBranchID).Consume(func(parentBranch *Branch) {
			if parentRejected = parentBranch.InclusionState() == Rejected; parentRejected {
				return
			}
		})

		if parentRejected {
			return errors.New("abort")
		}

		return nil
	})

	return
}

// anyConflictMemberConfirmed makes a Branch rejected if any of its conflicting Branches is
// Confirmed.
func (b *BranchDAG) anyConflictMemberConfirmed(branch *Branch) (conflictMemberConfirmed bool) {
	_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
		b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			if conflictMemberConfirmed || conflictMember.BranchID() == branch.ID() {
				return
			}

			b.Branch(conflictMember.BranchID()).Consume(func(conflictingBranch *Branch) {
				conflictMemberConfirmed = conflictingBranch.InclusionState() == Confirmed
			})
		})

		if conflictMemberConfirmed {
			return errors.New("abort")
		}

		return nil
	})

	return
}

// registerConflictMember is an internal utility function that creates the ConflictMember references of a Branch
// belonging to a given Conflict. It automatically creates the Conflict if it doesn't exist, yet.
func (b *BranchDAG) registerConflictMember(conflictID ConflictID, branchID *BranchID) {
	b.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) *Conflict {
		newConflict := NewConflict(conflictID)
		newConflict.Persist()
		newConflict.SetModified()

		return newConflict
	}).Consume(func(conflict *Conflict) {
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
	ID         *BranchID
	NewParents BranchIDs
}

func branchParentUpdateEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(branchParents *BranchParentUpdate))(params[0].(*BranchParentUpdate))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
