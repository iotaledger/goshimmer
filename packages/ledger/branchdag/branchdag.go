package branchdag

import (
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
	// inclusionStateMutex is a mutex that prevents that two processes simultaneously write the InclusionState.
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
		b.createChildBranchReferences(parentBranchIDs, branchID)

		if b.anyParentRejected(branch) || b.anyConflictingBranchConfirmed(branch) {
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

// AddBranchToConflicts adds the Branch to the named conflicts - it returns true if the conflict membership was modified
// during this operation.
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

// UpdateBranchParents changes the parents of a Branch after a fork (also updating the corresponding references).
func (b *BranchDAG) UpdateBranchParents(branchID, addedBranchID BranchID, removedBranchIDs BranchIDs) (updated bool) {
	b.inclusionStateMutex.RLock()
	b.Storage.CachedBranch(branchID).Consume(func(branch *Branch) {
		parentBranchIDs := branch.Parents()
		if !parentBranchIDs.Add(addedBranchID) {
			return
		}

		b.removeChildBranchReferences(parentBranchIDs.DeleteAll(removedBranchIDs), branchID)
		b.createChildBranchReferences(NewBranchIDs(addedBranchID), branchID)

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

// FilterPendingBranches takes a set of BranchIDs and removes all the Confirmed Branches (leaving only the pending or
// rejected ones behind).
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

// Shutdown shuts down the stateful elements of the BranchDAG (the Storage).
func (b *BranchDAG) Shutdown() {
	b.Storage.Shutdown()
}

// addConflictMembers creates the named ConflictMember references.
func (b *BranchDAG) addConflictMembers(branch *Branch, conflictIDs ConflictIDs) (added bool) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		if added = branch.AddConflict(conflictID); added {
			b.registerConflictMember(conflictID, branch.ID())
		}
	}

	return added
}

// createChildBranchReferences creates the named ChildBranch references.
func (b *BranchDAG) createChildBranchReferences(parentBranchIDs BranchIDs, childBranchID BranchID) {
	for it := parentBranchIDs.Iterator(); it.HasNext(); {
		if cachedChildBranch, stored := b.Storage.childBranchStorage.StoreIfAbsent(NewChildBranch(it.Next(), childBranchID)); stored {
			cachedChildBranch.Release()
		}
	}
}

// removeChildBranchReferences removes the named ChildBranch references.
func (b *BranchDAG) removeChildBranchReferences(parentBranchIDs BranchIDs, childBranchID BranchID) {
	for it := parentBranchIDs.Iterator(); it.HasNext(); {
		b.Storage.childBranchStorage.Delete(byteutils.ConcatBytes(it.Next().Bytes(), childBranchID.Bytes()))
	}
}

// anyParentRejected checks if any of a Branches parents is Rejected.
func (b *BranchDAG) anyParentRejected(branch *Branch) (rejected bool) {
	for it := branch.Parents().Iterator(); it.HasNext(); {
		if b.inclusionState(it.Next()) == Rejected {
			return true
		}
	}

	return false
}

// anyConflictingBranchConfirmed checks if any conflicting Branch is Confirmed.
func (b *BranchDAG) anyConflictingBranchConfirmed(branch *Branch) (anyConfirmed bool) {
	b.Utils.forEachConflictingBranchID(branch, func(conflictingBranchID BranchID) bool {
		anyConfirmed = b.inclusionState(conflictingBranchID) == Confirmed
		return !anyConfirmed
	})

	return anyConfirmed
}

// registerConflictMember registers a Branch in a Conflict by creating the references (if necessary) and increasing the
// corresponding member counter.
func (b *BranchDAG) registerConflictMember(conflictID ConflictID, branchID BranchID) {
	b.Storage.CachedConflict(conflictID, NewConflict).Consume(func(conflict *Conflict) {
		b.Storage.CachedConflictMember(conflictID, branchID, func(conflictID ConflictID, branchID BranchID) *ConflictMember {
			conflict.IncreaseMemberCount()
			return NewConflictMember(conflictID, branchID)
		}).Release()
	})
}

// inclusionState returns the InclusionState of the Branch with the given BranchID.
func (b *BranchDAG) inclusionState(branchID BranchID) (inclusionState InclusionState) {
	b.Storage.CachedBranch(branchID).Consume(func(branch *Branch) {
		inclusionState = branch.InclusionState()
	})

	return inclusionState
}
