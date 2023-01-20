package conflictdag

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

// ConflictDAG represents a generic DAG that is able to model causal dependencies between conflicts that try to access a
// shared set of resources.
type ConflictDAG[ConflictIDType, ResourceIDType comparable] struct {
	// Events contains the Events of the ConflictDAG.
	Events *Events[ConflictIDType, ResourceIDType]

	conflicts    *memstorage.Storage[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]]
	conflictSets *memstorage.Storage[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]]

	// mutex is a mutex that prevents that two processes simultaneously update the ConflictDAG.
	mutex sync.RWMutex

	optsMergeToMaster bool
}

// New is the constructor for the BlockDAG and creates a new BlockDAG instance.
func New[ConflictIDType, ResourceIDType comparable](opts ...options.Option[ConflictDAG[ConflictIDType, ResourceIDType]]) (c *ConflictDAG[ConflictIDType, ResourceIDType]) {
	return options.Apply(&ConflictDAG[ConflictIDType, ResourceIDType]{
		Events:            NewEvents[ConflictIDType, ResourceIDType](),
		conflicts:         memstorage.New[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]](),
		conflictSets:      memstorage.New[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]](),
		optsMergeToMaster: true,
	}, opts)
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) Conflict(conflictID ConflictIDType) (conflict *Conflict[ConflictIDType, ResourceIDType], exists bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.conflict(conflictID)
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) conflict(conflictID ConflictIDType) (conflict *Conflict[ConflictIDType, ResourceIDType], exists bool) {
	return c.conflicts.Get(conflictID)
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) ConflictSet(resourceID ResourceIDType) (conflictSet *ConflictSet[ConflictIDType, ResourceIDType], exists bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.conflictSet(resourceID)
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) conflictSet(resourceID ResourceIDType) (conflictSet *ConflictSet[ConflictIDType, ResourceIDType], exists bool) {
	return c.conflictSets.Get(resourceID)
}

// CreateConflict creates a new Conflict in the ConflictDAG and returns true if the Conflict was created.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) CreateConflict(id ConflictIDType, parentIDs *set.AdvancedSet[ConflictIDType], conflictingResourceIDs *set.AdvancedSet[ResourceIDType]) (created bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	conflictParents := set.NewAdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]()

	for it := parentIDs.Iterator(); it.HasNext(); {
		parentID := it.Next()
		parent, exists := c.conflict(parentID)
		if !exists {
			// if the parent does not exist it means that it has been evicted already. We can ignore it.
			continue
		}
		conflictParents.Add(parent)
	}

	conflict, created := c.conflicts.RetrieveOrCreate(id, func() (newConflict *Conflict[ConflictIDType, ResourceIDType]) {
		newConflict = NewConflict[ConflictIDType, ResourceIDType](id, parentIDs, set.NewAdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]())

		c.registerConflictWithConflictSet(newConflict, conflictingResourceIDs)

		// create parent references to newly created conflict
		for it := conflictParents.Iterator(); it.HasNext(); {
			it.Next().addChild(newConflict)
		}

		if c.anyParentRejected(conflictParents) || c.anyConflictingConflictAccepted(newConflict) {
			newConflict.setConfirmationState(confirmation.Rejected)
		}

		return
	})

	if created {
		c.Events.ConflictCreated.Trigger(conflict)
	}

	return created
}

// UpdateConflictParents changes the parents of a Conflict after a fork.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) UpdateConflictParents(id ConflictIDType, removedConflictIDs *set.AdvancedSet[ConflictIDType], addedConflictID ConflictIDType) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var parentConflictIDs *set.AdvancedSet[ConflictIDType]
	conflict, exists := c.conflict(id)
	if !exists {
		return false
	}

	parentConflictIDs = conflict.Parents()
	if !parentConflictIDs.Add(addedConflictID) {
		return
	}

	parentConflictIDs.DeleteAll(removedConflictIDs)

	conflict.setParents(parentConflictIDs)
	updated = true

	// create child reference in new parent
	if addedParent, parentExists := c.conflict(addedConflictID); parentExists {
		addedParent.addChild(conflict)

		if addedParent.ConfirmationState().IsRejected() && conflict.setConfirmationState(confirmation.Rejected) {
			c.Events.ConflictRejected.Trigger(conflict)
		}
	}

	// remove child references in deleted parents
	_ = removedConflictIDs.ForEach(func(conflictID ConflictIDType) (err error) {
		if removedParent, removedParentExists := c.conflict(conflictID); removedParentExists {
			removedParent.deleteChild(conflict)
		}
		return nil
	})

	if updated {
		c.Events.ConflictParentsUpdated.Trigger(&ConflictParentsUpdatedEvent[ConflictIDType, ResourceIDType]{
			ConflictID:         id,
			AddedConflict:      addedConflictID,
			RemovedConflicts:   removedConflictIDs,
			ParentsConflictIDs: parentConflictIDs,
		})
	}

	return updated
}

// UpdateConflictingResources adds the Conflict to the given ConflictSets - it returns true if the conflict membership was modified during this operation.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) UpdateConflictingResources(id ConflictIDType, conflictingResourceIDs *set.AdvancedSet[ResourceIDType]) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	conflict, exists := c.conflict(id)
	if !exists {
		return false
	}

	updated = c.registerConflictWithConflictSet(conflict, conflictingResourceIDs)

	if updated {
		c.Events.ConflictUpdated.Trigger(conflict)
	}

	return updated
}

// UnconfirmedConflicts takes a set of ConflictIDs and removes all the Accepted/Confirmed Conflicts (leaving only the
// pending or rejected ones behind).
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) UnconfirmedConflicts(conflictIDs *set.AdvancedSet[ConflictIDType]) (pendingConflictIDs *set.AdvancedSet[ConflictIDType]) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.optsMergeToMaster {
		return conflictIDs.Clone()
	}

	pendingConflictIDs = set.NewAdvancedSet[ConflictIDType]()
	for conflictWalker := conflictIDs.Iterator(); conflictWalker.HasNext(); {
		if currentConflictID := conflictWalker.Next(); !c.confirmationState(currentConflictID).IsAccepted() {
			pendingConflictIDs.Add(currentConflictID)
		}
	}

	return pendingConflictIDs
}

// SetConflictAccepted sets the ConfirmationState of the given Conflict to be Accepted - it automatically sets also the
// conflicting conflicts to be rejected.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) SetConflictAccepted(conflictID ConflictIDType) (modified bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Collect all resolved conflictSets and conflicts to delete them later
	conflictSets := set.NewAdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]()
	conflicts := set.NewAdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]()

	rejectionWalker := walker.New[ConflictIDType]()
	for confirmationWalker := set.NewAdvancedSet(conflictID).Iterator(); confirmationWalker.HasNext(); {
		currentConflictID := confirmationWalker.Next()
		conflict, exists := c.conflict(currentConflictID)
		if !exists {
			continue
		}
		conflicts.Add(conflict)
		conflictSets.AddAll(conflict.ConflictSets())

		if modified = conflict.setConfirmationState(confirmation.Accepted); !modified {
			continue
		}

		c.Events.ConflictAccepted.Trigger(conflict)

		confirmationWalker.PushAll(conflict.Parents().Slice()...)

		conflict.ForEachConflictingConflict(func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType]) bool {
			rejectionWalker.Push(conflictingConflict.ID())
			return true
		})
	}

	for rejectionWalker.HasNext() {
		conflict, exists := c.conflict(rejectionWalker.Next())
		if !exists {
			continue
		}
		conflicts.Add(conflict)
		conflictSets.AddAll(conflict.ConflictSets())

		if modified = conflict.setConfirmationState(confirmation.Rejected); !modified {
			continue
		}

		c.Events.ConflictRejected.Trigger(conflict)

		_ = conflict.Children().ForEach(func(childConflict *Conflict[ConflictIDType, ResourceIDType]) error {
			rejectionWalker.Push(childConflict.ID())
			return nil
		})
	}

	//// Delete all resolved ConflictSets (don't have a pending conflict anymore).
	//for it := conflictSets.Iterator(); it.HasNext(); {
	//	conflictSet := it.Next()
	//
	//	pendingConflicts := false
	//	for itConflict := conflictSet.Conflicts().Iterator(); itConflict.HasNext(); {
	//		conflict := itConflict.Next()
	//		if conflict.ConfirmationState() == confirmation.Pending {
	//			pendingConflicts = true
	//			continue
	//		}
	//		conflict.deleteConflictSet(conflictSet)
	//	}
	//
	//	if !pendingConflicts {
	//		c.conflictSets.Delete(conflictSet.ID())
	//	}
	//}
	//
	//// Delete all resolved Conflicts that are not part of any ConflictSet anymore.
	//for it := conflicts.Iterator(); it.HasNext(); {
	//	conflict := it.Next()
	//	if conflict.ConflictSets().Size() == 0 {
	//		c.conflicts.Delete(conflict.ID())
	//	}
	//}

	return modified
}

// ConfirmationState returns the ConfirmationState of the given ConflictIDs.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) ConfirmationState(conflictIDs *set.AdvancedSet[ConflictIDType]) (confirmationState confirmation.State) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	confirmationState = confirmation.Confirmed
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()
		if confirmationState = confirmationState.Aggregate(c.confirmationState(conflictID)); confirmationState.IsRejected() {
			return confirmation.Rejected
		}
		fmt.Println(conflictID, confirmationState)

	}
	return confirmationState
}

// DetermineVotes iterates over a set of conflicts and, taking into account the opinion a Voter expressed previously,
// computes the conflicts that will receive additional weight, the ones that will see their weight revoked, and if the
// result constitutes an overall valid state transition.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) DetermineVotes(conflictIDs *set.AdvancedSet[ConflictIDType]) (addedConflicts, revokedConflicts *set.AdvancedSet[ConflictIDType], isInvalid bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	addedConflicts = set.NewAdvancedSet[ConflictIDType]()
	for it := conflictIDs.Iterator(); it.HasNext(); {
		votedConflictID := it.Next()

		// The starting conflicts should not be considered as having common Parents, hence we treat them separately.
		conflictAddedConflicts, _ := c.determineConflictsToAdd(set.NewAdvancedSet(votedConflictID))
		addedConflicts.AddAll(conflictAddedConflicts)
	}
	revokedConflicts, isInvalid = c.determineConflictsToRevoke(addedConflicts)

	return
}

// determineConflictsToAdd iterates through the past cone of the given Conflicts and determines the ConflictIDs that
// are affected by the Vote.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) determineConflictsToAdd(conflictIDs *set.AdvancedSet[ConflictIDType]) (addedConflicts *set.AdvancedSet[ConflictIDType], allParentsAdded bool) {
	addedConflicts = set.NewAdvancedSet[ConflictIDType]()

	for it := conflictIDs.Iterator(); it.HasNext(); {
		currentConflictID := it.Next()

		conflict, exists := c.conflict(currentConflictID)
		if !exists {
			continue
		}

		addedConflictsOfCurrentConflict, allParentsOfCurrentConflictAdded := c.determineConflictsToAdd(conflict.Parents())
		allParentsAdded = allParentsAdded && allParentsOfCurrentConflictAdded

		addedConflicts.AddAll(addedConflictsOfCurrentConflict)

		addedConflicts.Add(currentConflictID)
	}

	return
}

// determineConflictsToRevoke determines which Conflicts of the conflicting future cone of the added Conflicts are affected
// by the vote and if the vote is valid (not voting for conflicting Conflicts).
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) determineConflictsToRevoke(addedConflicts *set.AdvancedSet[ConflictIDType]) (revokedConflicts *set.AdvancedSet[ConflictIDType], isInvalid bool) {
	revokedConflicts = set.NewAdvancedSet[ConflictIDType]()
	subTractionWalker := walker.New[ConflictIDType]()
	for it := addedConflicts.Iterator(); it.HasNext(); {
		conflict, exists := c.conflict(it.Next())
		if !exists {
			continue
		}

		conflict.ForEachConflictingConflict(func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType]) bool {
			subTractionWalker.Push(conflictingConflict.ID())

			return true
		})
	}

	for subTractionWalker.HasNext() {
		// currentVote := vote.WithConflictID(subTractionWalker.Next())
		//
		// if isInvalid = addedConflicts.Has(currentVote.ConflictID()) || votedConflicts.Has(currentVote.ConflictID()); isInvalid {
		//	return
		// }
		currentConflictID := subTractionWalker.Next()

		revokedConflicts.Add(currentConflictID)

		currentConflict, exists := c.conflict(currentConflictID)
		if !exists {
			continue
		}

		_ = currentConflict.Children().ForEach(func(childConflict *Conflict[ConflictIDType, ResourceIDType]) error {
			subTractionWalker.Push(childConflict.ID())
			return nil
		})
	}

	return
}

// anyParentRejected checks if any of a Conflicts parents is Rejected.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) anyParentRejected(parents *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]) (rejected bool) {
	for it := parents.Iterator(); it.HasNext(); {
		parent := it.Next()
		if parent.ConfirmationState().IsRejected() {
			return true
		}
	}

	return false
}

// anyConflictingConflictAccepted checks if any conflicting Conflict is Accepted/Confirmed.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) anyConflictingConflictAccepted(conflict *Conflict[ConflictIDType, ResourceIDType]) (anyAccepted bool) {
	conflict.ForEachConflictingConflict(func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType]) bool {
		anyAccepted = conflictingConflict.ConfirmationState().IsAccepted()
		return !anyAccepted
	})

	return anyAccepted
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) registerConflictWithConflictSet(conflict *Conflict[ConflictIDType, ResourceIDType], conflictingResourceIDs *set.AdvancedSet[ResourceIDType]) (added bool) {
	for it := conflictingResourceIDs.Iterator(); it.HasNext(); {
		conflictSetID := it.Next()

		conflictSet, _ := c.conflictSets.RetrieveOrCreate(conflictSetID, func() *ConflictSet[ConflictIDType, ResourceIDType] {
			return NewConflictSet[ConflictIDType, ResourceIDType](conflictSetID)
		})
		if conflict.addConflictSet(conflictSet) {
			conflictSet.AddConflictMember(conflict)
			added = true
		}
	}

	return added
}

// confirmationState returns the ConfirmationState of the Conflict with the given ConflictID.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) confirmationState(conflictID ConflictIDType) (confirmationState confirmation.State) {
	if conflict, exists := c.conflict(conflictID); exists {
		confirmationState = conflict.ConfirmationState()
	}

	return confirmationState
}

// ForEachConnectedConflictingConflictID executes the callback for each Conflict that is directly or indirectly connected to
// the named Conflict through a chain of intersecting conflicts.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) ForEachConnectedConflictingConflictID(rootConflict *Conflict[ConflictIDType, ResourceIDType], callback func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType])) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	traversedConflicts := set.New[*Conflict[ConflictIDType, ResourceIDType]]()
	conflictSetsWalker := walker.New[*ConflictSet[ConflictIDType, ResourceIDType]]()

	processConflictAndQueueConflictSets := func(conflict *Conflict[ConflictIDType, ResourceIDType]) {
		if !traversedConflicts.Add(conflict) {
			return
		}

		conflictSetsWalker.PushAll(conflict.ConflictSets().Slice()...)
	}

	processConflictAndQueueConflictSets(rootConflict)
	for conflictSetsWalker.HasNext() {
		conflictSet := conflictSetsWalker.Next()
		for it := conflictSet.Conflicts().Iterator(); it.HasNext(); {
			conflict := it.Next()
			processConflictAndQueueConflictSets(conflict)
		}
	}

	traversedConflicts.ForEach(callback)
}

// ForEachConflict iterates over every existing Conflict in the entire Storage.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) ForEachConflict(consumer func(conflict *Conflict[ConflictIDType, ResourceIDType])) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.conflicts.ForEach(func(c2 ConflictIDType, conflict *Conflict[ConflictIDType, ResourceIDType]) bool {
		consumer(conflict)
		return true
	})
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func MergeToMaster[ConflictIDType, ResourceIDType comparable](mergeToMaster bool) options.Option[ConflictDAG[ConflictIDType, ResourceIDType]] {
	return func(c *ConflictDAG[ConflictIDType, ResourceIDType]) {
		c.optsMergeToMaster = mergeToMaster
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
