package conflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/types/confirmation"
)

// ConflictDAG represents a generic DAG that is able to model causal dependencies between conflicts that try to access a
// shared set of resources.
type ConflictDAG[ConflictIDType, ResourceIDType comparable] struct {
	// Events is a dictionary for events emitted by the ConflictDAG.
	Events *Events[ConflictIDType, ResourceIDType]

	// Storage is a dictionary for storage related API endpoints.
	Storage *Storage[ConflictIDType, ResourceIDType]

	// Utils is a dictionary for utility methods that simplify the interaction with the ConflictDAG.
	Utils *Utils[ConflictIDType, ResourceIDType]

	// options is a dictionary for configuration parameters of the ConflictDAG.
	options *options

	// RWMutex is a mutex that prevents that two processes simultaneously update the ConfirmationState.
	sync.RWMutex
}

// New returns a new ConflictDAG with the given options.
func New[ConflictIDType, ResourceIDType comparable](options ...Option) (new *ConflictDAG[ConflictIDType, ResourceIDType]) {
	new = &ConflictDAG[ConflictIDType, ResourceIDType]{
		Events:  newEvents[ConflictIDType, ResourceIDType](),
		options: newOptions(options...),
	}
	new.Storage = newStorage[ConflictIDType, ResourceIDType](new.options)
	new.Utils = newUtils(new)

	return
}

// CreateConflict creates a new Conflict in the ConflictDAG and returns true if the Conflict was new.
func (b *ConflictDAG[ConflictIDType, ResourceIDType]) CreateConflict(id ConflictIDType, parents *set.AdvancedSet[ConflictIDType], conflictingResources *set.AdvancedSet[ResourceIDType]) (created bool) {
	b.RLock()
	b.Storage.CachedConflict(id, func(ConflictIDType) (conflict *Conflict[ConflictIDType, ResourceIDType]) {
		conflict = NewConflict(id, parents, set.NewAdvancedSet[ResourceIDType]())

		b.addConflictMembers(conflict, conflictingResources)
		b.createChildConflictReferences(parents, id)

		if b.anyParentRejected(conflict) || b.anyConflictingConflictAccepted(conflict) {
			conflict.setConfirmationState(confirmation.Rejected)
		}

		created = true

		return conflict
	}).Release()
	b.RUnlock()

	if created {
		b.Events.ConflictCreated.Trigger(&ConflictCreatedEvent[ConflictIDType, ResourceIDType]{
			ID:                     id,
			ParentConflictIDs:      parents,
			ConflictingResourceIDs: conflictingResources,
		})
	}

	return created
}

// UpdateConflictParents changes the parents of a Conflict after a fork (also updating the corresponding references).
func (b *ConflictDAG[ConflictIDType, ResourceIDType]) UpdateConflictParents(id ConflictIDType, removedConflictIDs *set.AdvancedSet[ConflictIDType], addedConflictID ConflictIDType) (updated bool) {
	b.RLock()

	var parentConflictIDs *set.AdvancedSet[ConflictIDType]
	b.Storage.CachedConflict(id).Consume(func(conflict *Conflict[ConflictIDType, ResourceIDType]) {
		parentConflictIDs = conflict.Parents()
		if !parentConflictIDs.Add(addedConflictID) {
			return
		}

		b.removeChildConflictReferences(parentConflictIDs.DeleteAll(removedConflictIDs), id)
		b.createChildConflictReferences(set.NewAdvancedSet(addedConflictID), id)

		conflict.SetParents(parentConflictIDs)
		updated = true
	})
	b.RUnlock()

	if updated {
		b.Events.ConflictParentsUpdated.Trigger(&ConflictParentsUpdatedEvent[ConflictIDType, ResourceIDType]{
			ConflictID:         id,
			AddedConflict:      addedConflictID,
			RemovedConflicts:   removedConflictIDs,
			ParentsConflictIDs: parentConflictIDs,
		})
	}

	return updated
}

// UpdateConflictingResources adds the Conflict to the named conflicts - it returns true if the conflict membership was modified
// during this operation.
func (b *ConflictDAG[ConflictIDType, ResourceIDType]) UpdateConflictingResources(id ConflictIDType, conflictingResourceIDs *set.AdvancedSet[ResourceIDType]) (updated bool) {
	b.RLock()
	b.Storage.CachedConflict(id).Consume(func(conflict *Conflict[ConflictIDType, ResourceIDType]) {
		updated = b.addConflictMembers(conflict, conflictingResourceIDs)
	})
	b.RUnlock()

	if updated {
		b.Events.ConflictConflictsUpdated.Trigger(&ConflictConflictsUpdatedEvent[ConflictIDType, ResourceIDType]{
			ConflictID:     id,
			NewConflictIDs: conflictingResourceIDs,
		})
	}

	return updated
}

// UnconfirmedConflicts takes a set of ConflictIDs and removes all the Accepted/Confirmed Conflicts (leaving only the
// pending or rejected ones behind).
func (b *ConflictDAG[ConflictIDType, ConflictingResourceID]) UnconfirmedConflicts(conflictIDs *set.AdvancedSet[ConflictIDType]) (pendingConflictIDs *set.AdvancedSet[ConflictIDType]) {
	if !b.options.mergeToMaster {
		return conflictIDs.Clone()
	}

	pendingConflictIDs = set.NewAdvancedSet[ConflictIDType]()
	for conflictWalker := conflictIDs.Iterator(); conflictWalker.HasNext(); {
		if currentConflictID := conflictWalker.Next(); !b.confirmationState(currentConflictID).IsAccepted() {
			pendingConflictIDs.Add(currentConflictID)
		}
	}

	return pendingConflictIDs
}

// SetConflictAccepted sets the ConfirmationState of the given Conflict to be Accepted - it automatically sets also the
// conflicting conflicts to be rejected.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) SetConflictAccepted(conflictID ConflictID) (modified bool) {
	b.Lock()
	defer b.Unlock()

	rejectionWalker := walker.New[ConflictID]()
	for confirmationWalker := set.NewAdvancedSet(conflictID).Iterator(); confirmationWalker.HasNext(); {
		b.Storage.CachedConflict(confirmationWalker.Next()).Consume(func(conflict *Conflict[ConflictID, ConflictingResourceID]) {
			if modified = conflict.setConfirmationState(confirmation.Accepted); !modified {
				return
			}

			b.Events.ConflictAccepted.Trigger(&ConflictAcceptedEvent[ConflictID]{
				ID: conflictID,
			})

			confirmationWalker.PushAll(conflict.Parents().Slice()...)

			b.Utils.forEachConflictingConflictID(conflict, func(conflictingConflictID ConflictID) bool {
				rejectionWalker.Push(conflictingConflictID)
				return true
			})
		})
	}

	for rejectionWalker.HasNext() {
		b.Storage.CachedConflict(rejectionWalker.Next()).Consume(func(conflict *Conflict[ConflictID, ConflictingResourceID]) {
			if modified = conflict.setConfirmationState(confirmation.Rejected); !modified {
				return
			}

			b.Events.ConflictRejected.Trigger(&ConflictRejectedEvent[ConflictID]{
				ID: conflict.ID(),
			})

			b.Storage.CachedChildConflicts(conflict.ID()).Consume(func(childConflict *ChildConflict[ConflictID]) {
				rejectionWalker.Push(childConflict.ChildConflictID())
			})
		})
	}

	return modified
}

// ConfirmationState returns the ConfirmationState of the given ConflictIDs.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) ConfirmationState(conflictIDs *set.AdvancedSet[ConflictID]) (confirmationState confirmation.State) {
	b.RLock()
	defer b.RUnlock()

	confirmationState = confirmation.Confirmed
	for it := conflictIDs.Iterator(); it.HasNext(); {
		if confirmationState = confirmationState.Aggregate(b.confirmationState(it.Next())); confirmationState.IsRejected() {
			return confirmation.Rejected
		}
	}

	return confirmationState
}

// Shutdown shuts down the stateful elements of the ConflictDAG (the Storage).
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) Shutdown() {
	b.Storage.Shutdown()
}

// addConflictMembers creates the named ConflictMember references.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) addConflictMembers(conflict *Conflict[ConflictID, ConflictingResourceID], conflictIDs *set.AdvancedSet[ConflictingResourceID]) (added bool) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		if added = conflict.addConflict(conflictID); added {
			b.registerConflictMember(conflictID, conflict.ID())
		}
	}

	return added
}

// createChildConflictReferences creates the named ChildConflict references.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) createChildConflictReferences(parentConflictIDs *set.AdvancedSet[ConflictID], childConflictID ConflictID) {
	for it := parentConflictIDs.Iterator(); it.HasNext(); {
		b.Storage.CachedChildConflict(it.Next(), childConflictID, NewChildConflict[ConflictID]).Release()
	}
}

// removeChildConflictReferences removes the named ChildConflict references.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) removeChildConflictReferences(parentConflictIDs *set.AdvancedSet[ConflictID], childConflictID ConflictID) {
	for it := parentConflictIDs.Iterator(); it.HasNext(); {
		b.Storage.childConflictStorage.Delete(byteutils.ConcatBytes(bytes(it.Next()), bytes(childConflictID)))
	}
}

// anyParentRejected checks if any of a Conflicts parents is Rejected.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) anyParentRejected(conflict *Conflict[ConflictID, ConflictingResourceID]) (rejected bool) {
	for it := conflict.Parents().Iterator(); it.HasNext(); {
		if b.confirmationState(it.Next()).IsRejected() {
			return true
		}
	}

	return false
}

// anyConflictingConflictAccepted checks if any conflicting Conflict is Accepted/Confirmed.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) anyConflictingConflictAccepted(conflict *Conflict[ConflictID, ConflictingResourceID]) (anyConfirmed bool) {
	b.Utils.forEachConflictingConflictID(conflict, func(conflictingConflictID ConflictID) bool {
		anyConfirmed = b.confirmationState(conflictingConflictID).IsAccepted()
		return !anyConfirmed
	})

	return anyConfirmed
}

// registerConflictMember registers a Conflict in a Conflict by creating the references (if necessary) and increasing the
// corresponding member counter.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) registerConflictMember(resourceID ConflictingResourceID, conflictID ConflictID) {
	b.Storage.CachedConflictMember(resourceID, conflictID, NewConflictMember[ConflictingResourceID, ConflictID]).Release()
}

// confirmationState returns the ConfirmationState of the Conflict with the given ConflictID.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) confirmationState(conflictID ConflictID) (confirmationState confirmation.State) {
	b.Storage.CachedConflict(conflictID).Consume(func(conflict *Conflict[ConflictID, ConflictingResourceID]) {
		confirmationState = conflict.ConfirmationState()
	})

	return confirmationState
}
