package ledgerstate

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region BranchDAG ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions of the ledger state that exist in the tangle.
type BranchDAG struct {
	branchStorage         *objectstorage.ObjectStorage[*Branch]
	childBranchStorage    *objectstorage.ObjectStorage[*ChildBranch]
	conflictStorage       *objectstorage.ObjectStorage[*Conflict]
	conflictMemberStorage *objectstorage.ObjectStorage[*ConflictMember]
	shutdownOnce          sync.Once
	Events                *BranchDAGEvents

	inclusionStateMutex sync.RWMutex
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(ledgerstate *Ledgerstate) (newBranchDAG *BranchDAG) {
	options := buildObjectStorageOptions(ledgerstate.Options.CacheTimeProvider)
	newBranchDAG = &BranchDAG{
		branchStorage:         objectstorage.New[*Branch](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixBranchStorage), options.branchStorageOptions...),
		childBranchStorage:    objectstorage.New[*ChildBranch](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixChildBranchStorage), options.childBranchStorageOptions...),
		conflictStorage:       objectstorage.New[*Conflict](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixConflictStorage), options.conflictStorageOptions...),
		conflictMemberStorage: objectstorage.New[*ConflictMember](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixConflictMemberStorage), options.conflictMemberStorageOptions...),
		Events: &BranchDAGEvents{
			BranchCreated:        events.NewEvent(BranchIDEventHandler),
			BranchParentsUpdated: events.NewEvent(branchParentUpdateEventCaller),
		},
	}
	newBranchDAG.init()

	return
}

// region CORE API /////////////////////////////////////////////////////////////////////////////////////////////////////

// CreateBranch retrieves the Branch that corresponds to the given details. It automatically creates and
// updates the Branch according to the new details if necessary.
func (b *BranchDAG) CreateBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (cachedBranch *objectstorage.CachedObject[*Branch], newBranchCreated bool, err error) {
	b.inclusionStateMutex.RLock()

	// create or load the branch
	cachedBranch = b.Branch(branchID, func() *Branch {
		branch := NewBranch(branchID, parentBranchIDs, conflictIDs)

		newBranchCreated = true

		return branch
	}).Retain()

	cachedBranch.Consume(func(branch *Branch) {
		// If the branch existed already we simply update its conflict members.
		//
		// An existing Branch can only become a new member of a conflict set if that conflict set was newly created in which
		// case none of the members of that set can either be Confirmed or Rejected. This means that our InclusionState does
		// not change, and we don't need to update and propagate it.
		if !newBranchCreated {
			for conflictID := range conflictIDs {
				if branch.AddConflict(conflictID) {
					b.registerConflictMember(conflictID, branchID)
				}
			}
			return
		}

		// store child references
		for resolvedParentBranchID := range parentBranchIDs {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(resolvedParentBranchID, branchID)); stored {
				cachedChildBranch.Release()
			}
		}

		// store ConflictMember references
		for conflictID := range conflictIDs {
			b.registerConflictMember(conflictID, branchID)
		}

		if b.anyParentRejected(branch) || b.anyConflictMemberConfirmed(branch) {
			branch.setInclusionState(Rejected)
		}
	})

	b.inclusionStateMutex.RUnlock()

	if newBranchCreated {
		b.Events.BranchCreated.Trigger(branchID)
	}

	return
}

// AddBranchParent changes the parents of a Branch (also updating the references of the ChildBranches).
func (b *BranchDAG) AddBranchParent(branchID, newParentBranchID BranchID) (err error) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	if !b.Branch(branchID).Consume(func(branch *Branch) {
		parentBranchIDs := branch.Parents()
		if _, exists := parentBranchIDs[newParentBranchID]; !exists {
			parentBranchIDs.Add(newParentBranchID)

			// make sure that once we add something, MasterBranchID is removed as it is the root of all branches.
			if parentBranchIDs.Contains(MasterBranchID) {
				delete(parentBranchIDs, MasterBranchID)
			}

			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(newParentBranchID, branchID)); stored {
				cachedChildBranch.Release()
			}

			if branch.SetParents(parentBranchIDs) {
				b.Events.BranchParentsUpdated.Trigger(&BranchParentUpdate{branchID, parentBranchIDs})
			}
		}
	}) {
		return errors.Errorf("failed to unwrap Branch: %w", cerrors.ErrFatal)
	}

	return nil
}

// ResolvePendingBranchIDs returns the BranchIDs of the pending and rejected Branches that are
// addressed by the given BranchIDs.
func (b *BranchDAG) ResolvePendingBranchIDs(branchIDs BranchIDs) (pendingBranchIDs BranchIDs, err error) {
	branchWalker := walker.New[BranchID]()
	for branchID := range branchIDs {
		branchWalker.Push(branchID)
	}

	pendingBranchIDs = make(BranchIDs)
	for branchWalker.HasNext() {
		currentBranchID := branchWalker.Next()

		if !b.Branch(currentBranchID).Consume(func(branch *Branch) {
			if branch.InclusionState() == Confirmed {
				return
			}
			pendingBranchIDs.Add(branch.ID())
		}) {
			return nil, errors.Errorf("failed to load Branch with %s: %w", currentBranchID, cerrors.ErrFatal)
		}
	}

	if len(pendingBranchIDs) == 0 {
		return NewBranchIDs(MasterBranchID), nil
	}

	return
}

// SetBranchConfirmed sets the InclusionState of the given Branch to be Confirmed.
func (b *BranchDAG) SetBranchConfirmed(branchID BranchID) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	confirmationWalker := walker.New[BranchID]().Push(branchID)
	rejectedWalker := walker.New[BranchID]()

	for confirmationWalker.HasNext() {
		currentBranchID := confirmationWalker.Next()

		b.Branch(currentBranchID).Consume(func(branch *Branch) {
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
	for branchID := range branchIDs {
		switch b.inclusionState(branchID) {
		case Rejected:
			return Rejected
		case Pending:
			inclusionState = Pending
		}
	}

	return inclusionState
}

// inclusionState returns the InclusionState of the given BranchID.
func (b *BranchDAG) inclusionState(branchID BranchID) (inclusionState InclusionState) {
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
func (b *BranchDAG) Branch(branchID BranchID, computeIfAbsentCallback ...func() *Branch) (cachedBranch *objectstorage.CachedObject[*Branch]) {
	if len(computeIfAbsentCallback) >= 1 {
		return b.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *Branch {
			return computeIfAbsentCallback[0]()
		})
	}

	return b.branchStorage.Load(branchID.Bytes())
}

// ChildBranches loads the references to the ChildBranches of the given Branch from the object storage.
func (b *BranchDAG) ChildBranches(branchID BranchID) (cachedChildBranches objectstorage.CachedObjects[*ChildBranch]) {
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
func (b *BranchDAG) ForEachConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID) bool) {
	abort := false
	b.Branch(branchID).Consume(func(branch *Branch) {
		for conflictID := range branch.Conflicts() {
			b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
				if abort || conflictMember.BranchID() == branchID {
					return
				}

				abort = !callback(conflictMember.BranchID())
			})
			if abort {
				return
			}
		}
	})
}

// ForEachConnectedConflictingBranchID executes the callback for each Branch that is connected through a chain
// of intersecting ConflictSets.
func (b *BranchDAG) ForEachConnectedConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID)) {
	traversedBranches := set.New[BranchID]()
	conflictSetsWalker := walker.New[ConflictID]()

	processBranchAndQueueConflictSets := func(branchID BranchID) {
		if !traversedBranches.Add(branchID) {
			return
		}

		b.Branch(branchID).Consume(func(branch *Branch) {
			for conflictID := range branch.Conflicts() {
				conflictSetsWalker.Push(conflictID)
			}
		})
	}

	processBranchAndQueueConflictSets(branchID)

	for conflictSetsWalker.HasNext() {
		b.ConflictMembers(conflictSetsWalker.Next()).Consume(func(conflictMember *ConflictMember) {
			processBranchAndQueueConflictSets(conflictMember.BranchID())
		})
	}

	traversedBranches.ForEach(func(element BranchID) {
		callback(element)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PRIVATE UTILITY FUNCTIONS ////////////////////////////////////////////////////////////////////////////////////

// init is an internal utility function that initializes the BranchDAG by creating the root of the DAG (MasterBranch).
func (b *BranchDAG) init() {
	cachedMasterBranch, stored := b.branchStorage.StoreIfAbsent(NewBranch(MasterBranchID, nil, nil))
	if stored {
		cachedMasterBranch.Consume(func(branch *Branch) {
			branch.setInclusionState(Confirmed)
		})
	}
}

func (b *BranchDAG) anyParentRejected(conflictBranch *Branch) (parentRejected bool) {
	for parentBranchID := range conflictBranch.Parents() {
		b.Branch(parentBranchID).Consume(func(parentBranch *Branch) {
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

// anyConflictMemberConfirmed makes a Branch rejected if any of its conflicting Branches is
// Confirmed.
func (b *BranchDAG) anyConflictMemberConfirmed(branch *Branch) (conflictMemberConfirmed bool) {
	for conflictID := range branch.Conflicts() {
		b.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			if conflictMemberConfirmed || conflictMember.BranchID() == branch.ID() {
				return
			}

			b.Branch(conflictMember.BranchID()).Consume(func(conflictingBranch *Branch) {
				conflictMemberConfirmed = conflictingBranch.InclusionState() == Confirmed
			})
		})

		if conflictMemberConfirmed {
			return
		}
	}

	return
}

// registerConflictMember is an internal utility function that creates the ConflictMember references of a Branch
// belonging to a given Conflict. It automatically creates the Conflict if it doesn't exist, yet.
func (b *BranchDAG) registerConflictMember(conflictID ConflictID, branchID BranchID) {
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
	ID         BranchID
	NewParents BranchIDs
}

func branchParentUpdateEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(branchParents *BranchParentUpdate))(params[0].(*BranchParentUpdate))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
