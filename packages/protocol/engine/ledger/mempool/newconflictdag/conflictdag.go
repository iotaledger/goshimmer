package newconflictdag

import (
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
		ConflictCreated:  event.New1[*conflict.Conflict[ConflictID, ResourceID]](),
		conflictsByID:    shrinkingmap.New[ConflictID, *conflict.Conflict[ConflictID, ResourceID]](),
		conflictSetsByID: shrinkingmap.New[ResourceID, *conflict.Set[ConflictID, ResourceID]](),
		pendingTasks:     syncutils.NewCounter(),
	}
}

// CreateConflict creates a new Conflict that is conflicting over the given ResourceIDs and that has the given parents.
func (c *ConflictDAG[ConflictID, ResourceID]) CreateConflict(id ConflictID, parentIDs []ConflictID, resourceIDs []ResourceID, initialWeight *weight.Weight) *conflict.Conflict[ConflictID, ResourceID] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	conflictSets := advancedset.New[*conflict.Set[ConflictID, ResourceID]]()
	for _, resourceID := range resourceIDs {
		conflictSets.Add(lo.Return1(c.conflictSetsByID.GetOrCreate(resourceID, func() *conflict.Set[ConflictID, ResourceID] {
			return conflict.NewSet[ConflictID, ResourceID](resourceID)
		})))
	}

	createdConflict, isNew := c.conflictsByID.GetOrCreate(id, func() *conflict.Conflict[ConflictID, ResourceID] {
		return conflict.New[ConflictID, ResourceID](id, c.Conflicts(parentIDs...), conflictSets, initialWeight, c.pendingTasks)
	})

	if !isNew {
		panic("tried to create a Conflict that already exists")
	}

	c.ConflictCreated.Trigger(createdConflict)

	return createdConflict
}

// Conflicts returns the Conflicts that are associated with the given ConflictIDs.
func (c *ConflictDAG[ConflictID, ResourceID]) Conflicts(ids ...ConflictID) *advancedset.AdvancedSet[*conflict.Conflict[ConflictID, ResourceID]] {
	conflicts := advancedset.New[*conflict.Conflict[ConflictID, ResourceID]]()
	for _, id := range ids {
		// TODO: check if it's okay to ignore non-existing conflicts
		if existingConflict, exists := c.conflictsByID.Get(id); exists {
			conflicts.Add(existingConflict)
		}
	}

	return conflicts
}

// ConflictSets returns the ConflictSets that are associated with the given ResourceIDs.
func (c *ConflictDAG[ConflictID, ResourceID]) ConflictSets(ids ...ResourceID) *advancedset.AdvancedSet[*conflict.Set[ConflictID, ResourceID]] {
	conflictSets := advancedset.New[*conflict.Set[ConflictID, ResourceID]]()
	for _, id := range ids {
		// TODO: check if it's okay to ignore non-existing conflictSets
		if existingConflictSet, exists := c.conflictSetsByID.Get(id); exists {
			conflictSets.Add(existingConflictSet)
		}
	}

	return conflictSets
}

// LikedInstead returns the ConflictIDs of the Conflicts that are liked instead of the Conflicts.
func (c *ConflictDAG[ConflictID, ResourceID]) LikedInstead(conflictIDs ...ConflictID) *advancedset.AdvancedSet[ConflictID] {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.pendingTasks.WaitIsZero()

	likedInstead := advancedset.New[ConflictID]()
	for _, conflictID := range conflictIDs {
		// TODO: discuss if it is okay to not find a conflict
		if existingConflict, exists := c.conflictsByID.Get(conflictID); exists {
			if largestConflict := c.largestConflict(existingConflict.LikedInstead()); largestConflict != nil {
				likedInstead.Add(largestConflict.ID())
			}
		}
	}

	return likedInstead
}

// largestConflict returns the largest Conflict from the given Conflicts.
func (c *ConflictDAG[ConflictID, ResourceID]) largestConflict(conflicts *advancedset.AdvancedSet[*conflict.Conflict[ConflictID, ResourceID]]) *conflict.Conflict[ConflictID, ResourceID] {
	var largestConflict *conflict.Conflict[ConflictID, ResourceID]

	_ = conflicts.ForEach(func(conflict *conflict.Conflict[ConflictID, ResourceID]) (err error) {
		if conflict.Compare(largestConflict) == weight.Heavier {
			largestConflict = conflict
		}

		return nil
	})

	return largestConflict
}
