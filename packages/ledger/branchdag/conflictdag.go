package branchdag

import (
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
)

// BranchDAG is an entity that manages conflicting versions of a quadruple-entry accounting ledger and their causal
// relationships.
type BranchDAG[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// Events is a dictionary for BranchDAG related events.
	Events *Events[ConflictID, ConflictSetID]

	// Storage is a dictionary for storage related API endpoints.
	Storage *Storage[ConflictID, ConflictSetID]

	// Utils is a dictionary for utility methods that simplify the interaction with the BranchDAG.
	Utils *Utils[ConflictID, ConflictSetID]

	// options is a dictionary for configuration parameters of the BranchDAG.
	options *options

	// inclusionStateMutex is a mutex that prevents that two processes simultaneously write the InclusionState.
	inclusionStateMutex sync.RWMutex
}

// New returns a new BranchDAG from the given options.
func New[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]](options ...Option) (new *BranchDAG[ConflictID, ConflictSetID]) {
	new = &BranchDAG[ConflictID, ConflictSetID]{
		Events:  newEvents[ConflictID, ConflictSetID](),
		options: newOptions(options...),
	}
	new.Storage = newStorage[ConflictID, ConflictSetID](new.options)
	new.Utils = newUtils(new)

	return
}

// CreateBranch tries to create a Branch with the given details. It returns true if the Branch could be created or false
// if it already existed. It triggers a BranchCreated event if the branch was successfully created.
func (b *BranchDAG[ConflictID, ConflictSetID]) CreateBranch(branchID ConflictID, parentBranchIDs *set.AdvancedSet[ConflictID], conflictIDs *set.AdvancedSet[ConflictSetID]) (created bool) {
	b.inclusionStateMutex.RLock()
	b.Storage.CachedBranch(branchID, func(ConflictID) (branch *Branch[ConflictID, ConflictSetID]) {
		branch = NewBranch(branchID, parentBranchIDs, set.NewAdvancedSet[ConflictSetID]())

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
		b.Events.BranchCreated.Trigger(&BranchCreatedEvent[ConflictID, ConflictSetID]{
			BranchID:        branchID,
			ParentBranchIDs: parentBranchIDs,
			ConflictIDs:     conflictIDs,
		})
	}

	return created
}

// AddBranchToConflicts adds the Branch to the named conflicts - it returns true if the conflict membership was modified
// during this operation.
func (b *BranchDAG[ConflictID, ConflictSetID]) AddBranchToConflicts(branchID ConflictID, newConflictIDs *set.AdvancedSet[ConflictSetID]) (updated bool) {
	b.inclusionStateMutex.RLock()
	b.Storage.CachedBranch(branchID).Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
		updated = b.addConflictMembers(branch, newConflictIDs)
	})
	b.inclusionStateMutex.RUnlock()

	if updated {
		b.Events.BranchConflictsUpdated.Trigger(&BranchConflictsUpdatedEvent[ConflictID, ConflictSetID]{
			BranchID:       branchID,
			NewConflictIDs: newConflictIDs,
		})
	}

	return updated
}

// UpdateBranchParents changes the parents of a Branch after a fork (also updating the corresponding references).
func (b *BranchDAG[ConflictID, ConflictSetID]) UpdateBranchParents(branchID, addedBranchID ConflictID, removedBranchIDs *set.AdvancedSet[ConflictID]) (updated bool) {
	b.inclusionStateMutex.RLock()

	var parentBranchIDs *set.AdvancedSet[ConflictID]
	b.Storage.CachedBranch(branchID).Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
		parentBranchIDs = branch.Parents()
		if !parentBranchIDs.Add(addedBranchID) {
			return
		}

		b.removeChildBranchReferences(parentBranchIDs.DeleteAll(removedBranchIDs), branchID)
		b.createChildBranchReferences(set.NewAdvancedSet(addedBranchID), branchID)

		updated = branch.SetParents(parentBranchIDs)
	})
	b.inclusionStateMutex.RUnlock()

	if updated {
		b.Events.BranchParentsUpdated.Trigger(&BranchParentsUpdatedEvent[ConflictID, ConflictSetID]{
			BranchID:         branchID,
			AddedBranch:      addedBranchID,
			RemovedBranches:  removedBranchIDs,
			ParentsBranchIDs: parentBranchIDs,
		})
	}

	return updated
}

// FilterPendingBranches takes a set of BranchIDs and removes all the Confirmed Branches (leaving only the pending or
// rejected ones behind).
func (b *BranchDAG[ConflictID, ConflictSetID]) FilterPendingBranches(branchIDs *set.AdvancedSet[ConflictID]) (pendingBranchIDs *set.AdvancedSet[ConflictID]) {
	if !b.options.mergeToMaster {
		return branchIDs.Clone()
	}

	pendingBranchIDs = set.NewAdvancedSet[ConflictID]()
	for branchWalker := branchIDs.Iterator(); branchWalker.HasNext(); {
		if currentBranchID := branchWalker.Next(); b.inclusionState(currentBranchID) != Confirmed {
			pendingBranchIDs.Add(currentBranchID)
		}
	}

	return pendingBranchIDs
}

// SetBranchConfirmed sets the InclusionState of the given Branch to be Confirmed - it automatically sets also the
// conflicting branches to be rejected.
func (b *BranchDAG[ConflictID, ConflictSetID]) SetBranchConfirmed(branchID ConflictID) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	rejectionWalker := walker.New[ConflictID]()
	for confirmationWalker := set.NewAdvancedSet(branchID).Iterator(); confirmationWalker.HasNext(); {
		b.Storage.CachedBranch(confirmationWalker.Next()).Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
			if modified = branch.setInclusionState(Confirmed); !modified {
				return
			}

			b.Events.BranchConfirmed.Trigger(&BranchConfirmedEvent[ConflictID]{
				BranchID: branchID,
			})

			confirmationWalker.PushAll(branch.Parents().Slice()...)

			b.Utils.forEachConflictingBranchID(branch, func(conflictingBranchID ConflictID) bool {
				rejectionWalker.Push(conflictingBranchID)
				return true
			})
		})
	}

	for rejectionWalker.HasNext() {
		b.Storage.CachedBranch(rejectionWalker.Next()).Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
			if modified = branch.setInclusionState(Rejected); !modified {
				return
			}

			b.Events.BranchRejected.Trigger(&BranchRejectedEvent[ConflictID]{
				BranchID: branch.ID(),
			})

			b.Storage.CachedChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch[ConflictID]) {
				rejectionWalker.Push(childBranch.ChildBranchID())
			})
		})
	}

	return modified
}

// InclusionState returns the InclusionState of the given BranchIDs.
func (b *BranchDAG[ConflictID, ConflictSetID]) InclusionState(branchIDs *set.AdvancedSet[ConflictID]) (inclusionState InclusionState) {
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
func (b *BranchDAG[ConflictID, ConflictSetID]) Shutdown() {
	b.Storage.Shutdown()
}

// addConflictMembers creates the named ConflictMember references.
func (b *BranchDAG[ConflictID, ConflictSetID]) addConflictMembers(branch *Branch[ConflictID, ConflictSetID], conflictIDs *set.AdvancedSet[ConflictSetID]) (added bool) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		if added = branch.addConflict(conflictID); added {
			b.registerConflictMember(conflictID, branch.ID())
		}
	}

	return added
}

// createChildBranchReferences creates the named ChildBranch references.
func (b *BranchDAG[ConflictID, ConflictSetID]) createChildBranchReferences(parentBranchIDs *set.AdvancedSet[ConflictID], childBranchID ConflictID) {
	for it := parentBranchIDs.Iterator(); it.HasNext(); {
		b.Storage.CachedChildBranch(it.Next(), childBranchID, NewChildBranch[ConflictID]).Release()
	}
}

// removeChildBranchReferences removes the named ChildBranch references.
func (b *BranchDAG[ConflictID, ConflictSetID]) removeChildBranchReferences(parentBranchIDs *set.AdvancedSet[ConflictID], childBranchID ConflictID) {
	for it := parentBranchIDs.Iterator(); it.HasNext(); {
		b.Storage.childBranchStorage.Delete(byteutils.ConcatBytes(it.Next().Bytes(), childBranchID.Bytes()))
	}
}

// anyParentRejected checks if any of a Branches parents is Rejected.
func (b *BranchDAG[ConflictID, ConflictSetID]) anyParentRejected(branch *Branch[ConflictID, ConflictSetID]) (rejected bool) {
	for it := branch.Parents().Iterator(); it.HasNext(); {
		if b.inclusionState(it.Next()) == Rejected {
			return true
		}
	}

	return false
}

// anyConflictingBranchConfirmed checks if any conflicting Branch is Confirmed.
func (b *BranchDAG[ConflictID, ConflictSetID]) anyConflictingBranchConfirmed(branch *Branch[ConflictID, ConflictSetID]) (anyConfirmed bool) {
	b.Utils.forEachConflictingBranchID(branch, func(conflictingBranchID ConflictID) bool {
		anyConfirmed = b.inclusionState(conflictingBranchID) == Confirmed
		return !anyConfirmed
	})

	return anyConfirmed
}

// registerConflictMember registers a Branch in a Conflict by creating the references (if necessary) and increasing the
// corresponding member counter.
func (b *BranchDAG[ConflictID, ConflictSetID]) registerConflictMember(conflictID ConflictSetID, branchID ConflictID) {
	b.Storage.CachedConflictMember(conflictID, branchID, NewConflictMember[ConflictID, ConflictSetID]).Release()
}

// inclusionState returns the InclusionState of the Branch with the given BranchID.
func (b *BranchDAG[ConflictID, ConflictSetID]) inclusionState(branchID ConflictID) (inclusionState InclusionState) {
	b.Storage.CachedBranch(branchID).Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
		inclusionState = branch.InclusionState()
	})

	return inclusionState
}
