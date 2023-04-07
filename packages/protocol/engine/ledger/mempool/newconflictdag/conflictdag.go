package newconflictdag

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

// ConflictDAG represents a data structure that tracks causal relationships between Conflicts and that allows to
// efficiently manage these Conflicts (and vote on their fate).
type ConflictDAG[ConflictID, ResourceID conflict.IDType] struct {
	// ConflictCreated is triggered when a new Conflict is created.
	ConflictCreated *event.Event1[*conflict.Conflict[ConflictID, ResourceID]]

	// ConflictingResourcesAdded is triggered when the Conflict is added to a new ConflictSet.
	ConflictingResourcesAdded *event.Event2[*conflict.Conflict[ConflictID, ResourceID], map[ResourceID]*conflict.Set[ConflictID, ResourceID]]

	// ConflictParentsUpdated is triggered when the parents of a Conflict are updated.
	ConflictParentsUpdated *event.Event3[*conflict.Conflict[ConflictID, ResourceID], *conflict.Conflict[ConflictID, ResourceID], []*conflict.Conflict[ConflictID, ResourceID]]

	// conflictsByID is a mapping of ConflictIDs to Conflicts.
	conflictsByID *shrinkingmap.ShrinkingMap[ConflictID, *conflict.Conflict[ConflictID, ResourceID]]

	// conflictSetsByID is a mapping of ResourceIDs to ConflictSets.
	conflictSetsByID *shrinkingmap.ShrinkingMap[ResourceID, *conflict.Set[ConflictID, ResourceID]]

	// pendingTasks is a counter that keeps track of the number of pending tasks.
	pendingTasks *syncutils.Counter

	// mutex is used to synchronize access to the ConflictDAG.
	mutex sync.RWMutex
}

// New creates a new ConflictDAG.
func New[ConflictID, ResourceID conflict.IDType]() *ConflictDAG[ConflictID, ResourceID] {
	return &ConflictDAG[ConflictID, ResourceID]{
		ConflictCreated:           event.New1[*conflict.Conflict[ConflictID, ResourceID]](),
		ConflictingResourcesAdded: event.New2[*conflict.Conflict[ConflictID, ResourceID], map[ResourceID]*conflict.Set[ConflictID, ResourceID]](),
		ConflictParentsUpdated:    event.New3[*conflict.Conflict[ConflictID, ResourceID], *conflict.Conflict[ConflictID, ResourceID], []*conflict.Conflict[ConflictID, ResourceID]](),
		conflictsByID:             shrinkingmap.New[ConflictID, *conflict.Conflict[ConflictID, ResourceID]](),
		conflictSetsByID:          shrinkingmap.New[ResourceID, *conflict.Set[ConflictID, ResourceID]](),
		pendingTasks:              syncutils.NewCounter(),
	}
}

// CreateConflict creates a new Conflict that is conflicting over the given ResourceIDs and that has the given parents.
func (c *ConflictDAG[ConflictID, ResourceID]) CreateConflict(id ConflictID, parentIDs []ConflictID, resourceIDs []ResourceID, initialWeight *weight.Weight) *conflict.Conflict[ConflictID, ResourceID] {
	createdConflict := func() *conflict.Conflict[ConflictID, ResourceID] {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		parents := lo.Values(c.Conflicts(parentIDs...))
		conflictSets := lo.Values(c.ConflictSets(resourceIDs...))

		if createdConflict, isNew := c.conflictsByID.GetOrCreate(id, func() *conflict.Conflict[ConflictID, ResourceID] {
			return conflict.New[ConflictID, ResourceID](id, parents, conflictSets, initialWeight, c.pendingTasks)
		}); isNew {
			return createdConflict
		}

		panic("tried to re-create an already existing conflict")
	}()

	c.ConflictCreated.Trigger(createdConflict)

	return createdConflict
}

// JoinConflictSets adds the Conflict to the given ConflictSets and returns true if the conflict membership was modified during this operation.
func (c *ConflictDAG[ConflictID, ResourceID]) JoinConflictSets(conflictID ConflictID, resourceIDs ...ResourceID) (joinedConflictSets map[ResourceID]*conflict.Set[ConflictID, ResourceID]) {
	currentConflict, joinedConflictSets := func() (*conflict.Conflict[ConflictID, ResourceID], map[ResourceID]*conflict.Set[ConflictID, ResourceID]) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, exists := c.conflictsByID.Get(conflictID)
		if !exists {
			return nil, nil
		}

		return currentConflict, currentConflict.JoinConflictSets(lo.Values(c.ConflictSets(resourceIDs...))...)
	}()

	if len(joinedConflictSets) > 0 {
		c.ConflictingResourcesAdded.Trigger(currentConflict, joinedConflictSets)
	}

	return joinedConflictSets
}

func (c *ConflictDAG[ConflictID, ResourceID]) UpdateConflictParents(conflictID ConflictID, addedParentID ConflictID, removedParentIDs ...ConflictID) bool {
	currentConflict, addedParent, removedParents, updated := func() (*conflict.Conflict[ConflictID, ResourceID], *conflict.Conflict[ConflictID, ResourceID], []*conflict.Conflict[ConflictID, ResourceID], bool) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, currentConflictExists := c.conflictsByID.Get(conflictID)
		addedParent, addedParentExists := c.Conflicts(addedParentID)[addedParentID]
		removedParents := lo.Values(c.Conflicts(removedParentIDs...))

		if !currentConflictExists || !addedParentExists {
			return nil, nil, nil, false
		}

		return currentConflict, addedParent, removedParents, currentConflict.UpdateParents(addedParent, removedParents...)
	}()

	if updated {
		c.ConflictParentsUpdated.Trigger(currentConflict, addedParent, removedParents)
	}

	return updated
}

// LikedInstead returns the ConflictIDs of the Conflicts that are liked instead of the Conflicts.
func (c *ConflictDAG[ConflictID, ResourceID]) LikedInstead(conflictIDs ...ConflictID) map[ConflictID]*conflict.Conflict[ConflictID, ResourceID] {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.pendingTasks.WaitIsZero()

	likedInstead := make(map[ConflictID]*conflict.Conflict[ConflictID, ResourceID])
	for _, conflictID := range conflictIDs {
		if currentConflict, exists := c.conflictsByID.Get(conflictID); exists {
			if largestConflict := largestConflict(currentConflict.LikedInstead()); largestConflict != nil {
				likedInstead[largestConflict.ID] = largestConflict
			}
		}
	}

	return likedInstead
}

// Conflicts returns the Conflicts that are associated with the given ConflictIDs.
func (c *ConflictDAG[ConflictID, ResourceID]) Conflicts(ids ...ConflictID) map[ConflictID]*conflict.Conflict[ConflictID, ResourceID] {
	conflicts := make(map[ConflictID]*conflict.Conflict[ConflictID, ResourceID])
	for _, id := range ids {
		if existingConflict, exists := c.conflictsByID.Get(id); exists {
			conflicts[id] = existingConflict
		}
	}

	return conflicts
}

// ConflictSets returns the ConflictSets that are associated with the given ResourceIDs.
func (c *ConflictDAG[ConflictID, ResourceID]) ConflictSets(resourceIDs ...ResourceID) map[ResourceID]*conflict.Set[ConflictID, ResourceID] {
	conflictSets := make(map[ResourceID]*conflict.Set[ConflictID, ResourceID])
	for _, resourceID := range resourceIDs {
		conflictSets[resourceID] = lo.Return1(c.conflictSetsByID.GetOrCreate(resourceID, func() *conflict.Set[ConflictID, ResourceID] {
			return conflict.NewSet[ConflictID, ResourceID](resourceID)
		}))
	}

	return conflictSets
}

func (c *ConflictDAG[ConflictID, ResourceID]) CastVotes(conflictIDs ...ConflictID) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	conflictsToAdd := c.determineConflictsToAdd(advancedset.New[*conflict.Conflict[ConflictID, ResourceID]](lo.Values(c.Conflicts(conflictIDs...))...))

	if false {
		fmt.Println(conflictsToAdd)
	}

	// revokedConflicts, isInvalid = c.determineConflictsToRevoke(conflictsToAdd)
}

// determineConflictsToAdd iterates through the past cone of the given Conflicts and determines the ConflictIDs that
// are affected by the Vote.
func (c *ConflictDAG[ConflictID, ResourceID]) determineConflictsToAdd(conflictIDs *advancedset.AdvancedSet[*conflict.Conflict[ConflictID, ResourceID]]) (addedConflicts *advancedset.AdvancedSet[*conflict.Conflict[ConflictID, ResourceID]]) {
	addedConflicts = advancedset.New[*conflict.Conflict[ConflictID, ResourceID]]()
	for it := conflictIDs.Iterator(); it.HasNext(); {
		currentConflict := it.Next()

		addedConflicts.AddAll(c.determineConflictsToAdd(currentConflict.Parents))
		addedConflicts.Add(currentConflict)
	}

	return
}
