package conflictdag

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

// ConflictDAG represents a generic DAG that is able to model causal dependencies between conflicts that try to access a
// shared set of resources.
type ConflictDAG[ConflictIDType, ResourceIDType comparable] struct {
	// Events contains the Events of the ConflictDAG.
	Events *Events[ConflictIDType, ResourceIDType]

	conflicts    *shrinkingmap.ShrinkingMap[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]]
	conflictSets *shrinkingmap.ShrinkingMap[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]]

	// mutex is a mutex that prevents that two processes simultaneously update the ConflictDAG.
	mutex *syncutils.StarvingMutex

	// WeightsMutex is a mutex that prevents updating conflict weights when creating references for a new block.
	// It is used by different components, but it is placed here because it's easily accessible in all needed components.
	// It serves more as a quick-fix, as eventually conflict tracking spread across multiple components
	// (ConflictDAG, ConflictResolver, ConflictsTracker) will be refactored into a single component that handles locking nicely.
	WeightsMutex sync.RWMutex

	optsMergeToMaster bool
}

// New is the constructor for the BlockDAG and creates a new BlockDAG instance.
func New[ConflictIDType, ResourceIDType comparable](opts ...options.Option[ConflictDAG[ConflictIDType, ResourceIDType]]) (c *ConflictDAG[ConflictIDType, ResourceIDType]) {
	return options.Apply(&ConflictDAG[ConflictIDType, ResourceIDType]{
		Events:            NewEvents[ConflictIDType, ResourceIDType](),
		conflicts:         shrinkingmap.New[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]](),
		conflictSets:      shrinkingmap.New[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]](),
		mutex:             syncutils.NewStarvingMutex(),
		optsMergeToMaster: true,
	}, opts)
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) Conflict(conflictID ConflictIDType) (conflict *Conflict[ConflictIDType, ResourceIDType], exists bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.conflicts.Get(conflictID)
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) ConflictSet(resourceID ResourceIDType) (conflictSet *ConflictSet[ConflictIDType, ResourceIDType], exists bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.conflictSets.Get(resourceID)
}

// CreateConflict creates a new Conflict in the ConflictDAG and returns true if the Conflict was created.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) CreateConflict(id ConflictIDType, parentIDs *advancedset.AdvancedSet[ConflictIDType], conflictingResourceIDs *advancedset.AdvancedSet[ResourceIDType], confirmationState confirmation.State) (created bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	conflictParents := advancedset.New[*Conflict[ConflictIDType, ResourceIDType]]()
	for it := parentIDs.Iterator(); it.HasNext(); {
		parentID := it.Next()
		parent, exists := c.conflicts.Get(parentID)
		if !exists {
			// if the parent does not exist it means that it has been evicted already. We can ignore it.
			continue
		}
		conflictParents.Add(parent)
	}

	conflict, created := c.conflicts.GetOrCreate(id, func() (newConflict *Conflict[ConflictIDType, ResourceIDType]) {
		newConflict = NewConflict(id, parentIDs, advancedset.New[*ConflictSet[ConflictIDType, ResourceIDType]](), confirmationState)

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
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) UpdateConflictParents(id ConflictIDType, removedConflictIDs *advancedset.AdvancedSet[ConflictIDType], addedConflictID ConflictIDType) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var parentConflictIDs *advancedset.AdvancedSet[ConflictIDType]
	conflict, exists := c.conflicts.Get(id)
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
	if addedParent, parentExists := c.conflicts.Get(addedConflictID); parentExists {
		addedParent.addChild(conflict)

		if addedParent.ConfirmationState().IsRejected() && conflict.setConfirmationState(confirmation.Rejected) {
			c.Events.ConflictRejected.Trigger(conflict)
		}
	}

	// remove child references in deleted parents
	_ = removedConflictIDs.ForEach(func(conflictID ConflictIDType) (err error) {
		if removedParent, removedParentExists := c.conflicts.Get(conflictID); removedParentExists {
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
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) UpdateConflictingResources(id ConflictIDType, conflictingResourceIDs *advancedset.AdvancedSet[ResourceIDType]) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	conflict, exists := c.conflicts.Get(id)
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
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) UnconfirmedConflicts(conflictIDs *advancedset.AdvancedSet[ConflictIDType]) (pendingConflictIDs *advancedset.AdvancedSet[ConflictIDType]) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.optsMergeToMaster {
		return conflictIDs.Clone()
	}

	pendingConflictIDs = advancedset.New[ConflictIDType]()
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

	conflictsToReject := advancedset.New[*Conflict[ConflictIDType, ResourceIDType]]()

	for confirmationWalker := advancedset.New(conflictID).Iterator(); confirmationWalker.HasNext(); {
		currentConflictID := confirmationWalker.Next()
		conflict, exists := c.conflicts.Get(currentConflictID)
		if !exists {
			continue
		}

		if conflict.ConfirmationState() != confirmation.NotConflicting {
			if !conflict.setConfirmationState(confirmation.Accepted) {
				continue
			}

			modified = true

			c.Events.ConflictAccepted.Trigger(conflict)
		}

		confirmationWalker.PushAll(conflict.Parents().Slice()...)

		conflict.ForEachConflictingConflict(func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType]) bool {
			conflictsToReject.Add(conflictingConflict)
			return true
		})
	}

	modified = c.rejectConflictsWithFutureCone(conflictsToReject) || modified

	// // Delete all resolved ConflictSets (don't have a pending conflict anymore).
	// for it := conflictSets.Iterator(); it.HasNext(); {
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
	// }
	//
	// // Delete all resolved Conflicts that are not part of any ConflictSet anymore.
	// for it := conflicts.Iterator(); it.HasNext(); {
	//	conflict := it.Next()
	//	if conflict.ConflictSets().Size() == 0 {
	//		c.conflicts.Delete(conflict.ID())
	//	}
	// }

	return modified
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) rejectConflictsWithFutureCone(initialConflicts *advancedset.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]) (modified bool) {
	rejectionWalker := walker.New[*Conflict[ConflictIDType, ResourceIDType]]().PushAll(initialConflicts.Slice()...)
	for rejectionWalker.HasNext() {
		conflict := rejectionWalker.Next()
		if !conflict.setConfirmationState(confirmation.Rejected) {
			continue
		}

		modified = true

		c.Events.ConflictRejected.Trigger(conflict)
		rejectionWalker.PushAll(conflict.Children().Slice()...)
	}

	return modified
}

// ConfirmationState returns the ConfirmationState of the given ConflictIDs.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) ConfirmationState(conflictIDs *advancedset.AdvancedSet[ConflictIDType]) (confirmationState confirmation.State) {
	// we are on master reality.
	if conflictIDs.IsEmpty() {
		return confirmation.Confirmed
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// we start with Confirmed because state is Aggregated to the lowest state.
	confirmationState = confirmation.Confirmed
	for conflictID := conflictIDs.Iterator(); conflictID.HasNext(); {
		if confirmationState = confirmationState.Aggregate(c.confirmationState(conflictID.Next())); confirmationState.IsRejected() {
			return confirmation.Rejected
		}
	}

	return confirmationState
}

// DetermineVotes iterates over a set of conflicts and, taking into account the opinion a Voter expressed previously,
// computes the conflicts that will receive additional weight, the ones that will see their weight revoked, and if the
// result constitutes an overall valid state transition.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) DetermineVotes(conflictIDs *advancedset.AdvancedSet[ConflictIDType]) (addedConflicts, revokedConflicts *advancedset.AdvancedSet[ConflictIDType], isInvalid bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	addedConflicts = advancedset.New[ConflictIDType]()
	for it := conflictIDs.Iterator(); it.HasNext(); {
		votedConflictID := it.Next()

		// The starting conflicts should not be considered as having common Parents, hence we treat them separately.
		conflictAddedConflicts, _ := c.determineConflictsToAdd(advancedset.New(votedConflictID))
		addedConflicts.AddAll(conflictAddedConflicts)
	}
	revokedConflicts, isInvalid = c.determineConflictsToRevoke(addedConflicts)

	return
}

// determineConflictsToAdd iterates through the past cone of the given Conflicts and determines the ConflictIDs that
// are affected by the Vote.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) determineConflictsToAdd(conflictIDs *advancedset.AdvancedSet[ConflictIDType]) (addedConflicts *advancedset.AdvancedSet[ConflictIDType], allParentsAdded bool) {
	addedConflicts = advancedset.New[ConflictIDType]()

	for it := conflictIDs.Iterator(); it.HasNext(); {
		currentConflictID := it.Next()

		conflict, exists := c.conflicts.Get(currentConflictID)
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
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) determineConflictsToRevoke(addedConflicts *advancedset.AdvancedSet[ConflictIDType]) (revokedConflicts *advancedset.AdvancedSet[ConflictIDType], isInvalid bool) {
	revokedConflicts = advancedset.New[ConflictIDType]()
	subTractionWalker := walker.New[ConflictIDType]()
	for it := addedConflicts.Iterator(); it.HasNext(); {
		conflict, exists := c.conflicts.Get(it.Next())
		if !exists {
			continue
		}

		conflict.ForEachConflictingConflict(func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType]) bool {
			subTractionWalker.Push(conflictingConflict.ID())

			return true
		})
	}

	for subTractionWalker.HasNext() {
		currentConflictID := subTractionWalker.Next()

		if isInvalid = addedConflicts.Has(currentConflictID); isInvalid {
			fmt.Println("block is subjectively invalid because of conflict", currentConflictID)

			return revokedConflicts, true
		}

		revokedConflicts.Add(currentConflictID)

		currentConflict, exists := c.conflicts.Get(currentConflictID)
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
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) anyParentRejected(parents *advancedset.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]) (rejected bool) {
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

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) registerConflictWithConflictSet(conflict *Conflict[ConflictIDType, ResourceIDType], conflictingResourceIDs *advancedset.AdvancedSet[ResourceIDType]) (added bool) {
	for it := conflictingResourceIDs.Iterator(); it.HasNext(); {
		conflictSetID := it.Next()

		conflictSet, _ := c.conflictSets.GetOrCreate(conflictSetID, func() *ConflictSet[ConflictIDType, ResourceIDType] {
			return NewConflictSet[ConflictIDType](conflictSetID)
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
	if conflict, exists := c.conflicts.Get(conflictID); exists {
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

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) HandleOrphanedConflict(conflictID ConflictIDType) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	initialConflict, exists := c.conflicts.Get(conflictID)
	if !exists {
		return
	}

	c.rejectConflictsWithFutureCone(advancedset.New(initialConflict))

	// iterate conflict's conflictSets. if only one conflict is pending, then mark it appropriately
	for it := initialConflict.conflictSets.Iterator(); it.HasNext(); {
		conflictSet := it.Next()

		pendingConflict := c.getLastPendingConflict(conflictSet)
		if pendingConflict == nil {
			continue
		}

		// check if the last pending conflict is part of any other ConflictSets need to be voted on.
		nonResolvedConflictSets := false
		for pendingConflictSetsIt := pendingConflict.ConflictSets().Iterator(); pendingConflictSetsIt.HasNext(); {
			pendingConflictConflictSet := pendingConflictSetsIt.Next()
			if lastConflictSetElement := c.getLastPendingConflict(pendingConflictConflictSet); lastConflictSetElement == nil {
				nonResolvedConflictSets = true
				break
			}
		}

		// if pendingConflict does not belong to any pending conflict sets, mark it as NotConflicting.
		if !nonResolvedConflictSets {
			pendingConflict.setConfirmationState(confirmation.NotConflicting)
			c.Events.ConflictNotConflicting.Trigger(pendingConflict)
		}
	}
}

// getLastPendingConflict returns last pending Conflict from the ConflictSet or returns nil if zero or more than one pending conflicts left.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) getLastPendingConflict(conflictSet *ConflictSet[ConflictIDType, ResourceIDType]) (pendingConflict *Conflict[ConflictIDType, ResourceIDType]) {
	pendingConflictsCount := 0

	for itConflict := conflictSet.Conflicts().Iterator(); itConflict.HasNext(); {
		conflictSetMember := itConflict.Next()

		if conflictSetMember.ConfirmationState() == confirmation.Pending {
			pendingConflict = conflictSetMember
			pendingConflictsCount++
		}

		if pendingConflictsCount > 1 {
			return nil
		}
	}

	return pendingConflict
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func MergeToMaster[ConflictIDType, ResourceIDType comparable](mergeToMaster bool) options.Option[ConflictDAG[ConflictIDType, ResourceIDType]] {
	return func(c *ConflictDAG[ConflictIDType, ResourceIDType]) {
		c.optsMergeToMaster = mergeToMaster
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
