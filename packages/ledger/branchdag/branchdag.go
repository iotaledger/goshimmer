package branchdag

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/walker"
)

// BranchDAG is an entity that manages conflicting versions of a quadruple-entry-accounting ledger and their causal
// relationships.
type BranchDAG struct {
	// Events is a dictionary for BranchDAG related events.
	Events *Events
	// Storage is a dictionary for storage related API endpoints.
	Storage *Storage
	// Utils is a dictionary for utility methods that simplify the interaction with the BranchDAG.
	Utils *Utils
	// options is a dictionary for configuration parameters of the BranchDAG.
	options *options
	// inclusionStateMutex is a mutex that prevents that two .
	inclusionStateMutex sync.RWMutex
}

// New returns a new BranchDAG from the given options.
func New(options ...Option) (new *BranchDAG) {
	new = &BranchDAG{
		Events:  newEvents(),
		options: newOptions(options...),
	}
	new.Storage = newStorage(new)
	new.Utils = newUtils(new)

	return
}

// CreateBranch tries to create a Branch with the given details. It returns true if the Branch could be created or false
// if it already existed. It triggers a BranchCreated event if the branch was successfully created.
func (b *BranchDAG) CreateBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (created bool) {
	b.inclusionStateMutex.RLock()
	b.Storage.CachedBranch(branchID, func() (branch *Branch) {
		branch = NewBranch(branchID, parentBranchIDs, NewConflictIDs())

		b.addConflictMembers(branch, conflictIDs)
		b.createChildBranchReferences(branchID, parentBranchIDs)

		if b.anyParentRejected(branch) || b.anyConflictMemberConfirmed(branch) {
			branch.setInclusionState(Rejected)
		}

		created = true

		return branch
	}).Release()
	b.inclusionStateMutex.RUnlock()

	if created {
		b.Events.BranchCreated.Trigger(&BranchCreatedEvent{
			BranchID:        branchID,
			ParentBranchIDs: parentBranchIDs,
			ConflictIDs:     conflictIDs,
		})
	}

	return created
}

// AddBranchToConflicts adds the given Branch to the named conflicts.
func (b *BranchDAG) AddBranchToConflicts(branchID BranchID, newConflictIDs ConflictIDs) (updated bool) {
	b.inclusionStateMutex.RLock()
	b.Storage.CachedBranch(branchID).Consume(func(branch *Branch) {
		updated = b.addConflictMembers(branch, newConflictIDs)
	})
	b.inclusionStateMutex.RUnlock()

	if updated {
		b.Events.BranchConflictsUpdated.Trigger(&BranchConflictsUpdatedEvent{
			BranchID:       branchID,
			NewConflictIDs: newConflictIDs,
		})
	}

	return updated
}

// UpdateBranchParents changes the parents of a Branch (also updating the references of the CachedChildBranches).
func (b *BranchDAG) UpdateBranchParents(branchID, addedBranchID BranchID, removedBranchIDs BranchIDs) (updated bool) {
	b.inclusionStateMutex.RLock()
	b.Storage.CachedBranch(branchID).Consume(func(branch *Branch) {
		parentBranchIDs := branch.Parents()
		if !parentBranchIDs.Add(addedBranchID) {
			return
		}

		b.removeChildBranchReferences(parentBranchIDs.DeleteAll(removedBranchIDs), branchID)
		b.createChildBranchReferences(branchID, NewBranchIDs(addedBranchID))

		updated = branch.SetParents(parentBranchIDs)
	})
	b.inclusionStateMutex.RUnlock()

	if updated {
		b.Events.BranchParentsUpdated.Trigger(&BranchParentsUpdatedEvent{
			BranchID:        branchID,
			AddedBranch:     addedBranchID,
			RemovedBranches: removedBranchIDs,
		})
	}

	return updated
}

// FilterPendingBranches returns the BranchIDs of the pending and rejected Branches that are
// addressed by the given BranchIDs.
func (b *BranchDAG) FilterPendingBranches(branchIDs BranchIDs) (pendingBranchIDs BranchIDs) {
	pendingBranchIDs = NewBranchIDs()
	for branchWalker := branchIDs.Iterator(); branchWalker.HasNext(); {
		if currentBranchID := branchWalker.Next(); b.inclusionState(currentBranchID) != Confirmed {
			pendingBranchIDs.Add(currentBranchID)
		}
	}

	return pendingBranchIDs
}

// SetBranchConfirmed sets the InclusionState of the given Branch to be Confirmed.
func (b *BranchDAG) SetBranchConfirmed(branchID BranchID) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	confirmationWalker := walker.New[BranchID]().Push(branchID)
	rejectedWalker := walker.New[BranchID]()

	for confirmationWalker.HasNext() {
		currentBranchID := confirmationWalker.Next()

		b.Storage.CachedBranch(currentBranchID).Consume(func(branch *Branch) {
			if modified = branch.setInclusionState(Confirmed); !modified {
				return
			}

			for it := branch.Parents().Iterator(); it.HasNext(); {
				confirmationWalker.Push(it.Next())
			}

			for it := branch.Conflicts().Iterator(); it.HasNext(); {
				b.Storage.CachedConflictMembers(it.Next()).Consume(func(conflictMember *ConflictMember) {
					if conflictMember.BranchID() != currentBranchID {
						rejectedWalker.Push(conflictMember.BranchID())
					}
				})
			}
		})
	}

	for rejectedWalker.HasNext() {
		b.Storage.CachedBranch(rejectedWalker.Next()).Consume(func(branch *Branch) {
			if modified = branch.setInclusionState(Rejected); !modified {
				return
			}

			b.Storage.CachedChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
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
	for it := branchIDs.Iterator(); it.HasNext(); {
		switch b.inclusionState(it.Next()) {
		case Rejected:
			return Rejected
		case Pending:
			inclusionState = Pending
		}
	}

	return inclusionState
}

func (b *BranchDAG) Shutdown() {
	b.Storage.Shutdown()
}

func (b *BranchDAG) addConflictMembers(branch *Branch, conflictIDs ConflictIDs) (added bool) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		if added = branch.AddConflict(conflictID); added {
			b.registerConflictMember(conflictID, branch.ID())
		}
	}

	return added
}

func (b *BranchDAG) createChildBranchReferences(branchID BranchID, parentBranchIDs BranchIDs) {
	for it := parentBranchIDs.Iterator(); it.HasNext(); {
		if cachedChildBranch, stored := b.Storage.childBranchStorage.StoreIfAbsent(NewChildBranch(it.Next(), branchID)); stored {
			cachedChildBranch.Release()
		}
	}
}

func (b *BranchDAG) removeChildBranchReferences(parentBranches BranchIDs, childBranchID BranchID) {
	for it := parentBranches.Iterator(); it.HasNext(); {
		b.Storage.childBranchStorage.Delete(byteutils.ConcatBytes(it.Next().Bytes(), childBranchID.Bytes()))
	}
}

// anyParentRejected checks if any of a Branches parents is rejected.
func (b *BranchDAG) anyParentRejected(branch *Branch) (rejected bool) {
	for it := branch.Parents().Iterator(); it.HasNext(); {
		if b.inclusionState(it.Next()) == Rejected {
			return true
		}
	}

	return false
}

// anyConflictMemberConfirmed checks if any of a Branch's conflicts is Confirmed.
func (b *BranchDAG) anyConflictMemberConfirmed(branch *Branch) (confirmed bool) {
	b.Utils.forEachConflictingBranchID(branch, func(conflictingBranchID BranchID) bool {
		confirmed = b.inclusionState(conflictingBranchID) == Confirmed
		return !confirmed
	})

	return confirmed
}

// registerConflictMember registers a Branch in a Conflict by creating the references (if necessary) and increasing the
// member counter of the Conflict.
func (b *BranchDAG) registerConflictMember(conflictID ConflictID, branchID BranchID) {
	b.Storage.CachedConflict(conflictID, NewConflict).Consume(func(conflict *Conflict) {
		b.Storage.CachedConflictMember(conflictID, branchID, func(conflictID ConflictID, branchID BranchID) *ConflictMember {
			conflict.IncreaseMemberCount()
			return NewConflictMember(conflictID, branchID)
		}).Release()
	})
}

// inclusionState returns the InclusionState of the given BranchID.
func (b *BranchDAG) inclusionState(branchID BranchID) (inclusionState InclusionState) {
	if !b.Storage.CachedBranch(branchID).Consume(func(branch *Branch) {
		inclusionState = branch.InclusionState()
	}) {
		panic(fmt.Sprintf("failed to load %s", branchID))
	}

	return inclusionState
}
