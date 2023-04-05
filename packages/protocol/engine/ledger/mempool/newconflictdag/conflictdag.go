package newconflictdag

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
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

	parents := c.Conflicts(parentIDs...)
	conflictSets := c.ConflictSets(resourceIDs...)

	createdConflict, isNew := c.conflictsByID.GetOrCreate(id, func() *conflict.Conflict[ConflictID, ResourceID] {
		return conflict.New[ConflictID, ResourceID](id, parents, conflictSets, initialWeight, c.pendingTasks)
	})

	if !isNew {
		panic("tried to create a Conflict that already exists")
	}

	c.ConflictCreated.Trigger(createdConflict)

	return createdConflict
}

// LikedInstead returns the ConflictIDs of the Conflicts that are liked instead of the Conflicts.
func (c *ConflictDAG[ConflictID, ResourceID]) LikedInstead(conflictIDs ...ConflictID) []ConflictID {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.pendingTasks.WaitIsZero()

	likedInstead := make([]ConflictID, 0)
	for _, conflictID := range conflictIDs {
		if currentConflict, exists := c.conflictsByID.Get(conflictID); exists {
			if largestConflict := largestConflict(currentConflict.LikedInstead()); largestConflict != nil {
				likedInstead = append(likedInstead, largestConflict.ID())
			}
		}
	}

	return likedInstead
}

// Conflicts returns the Conflicts that are associated with the given ConflictIDs.
func (c *ConflictDAG[ConflictID, ResourceID]) Conflicts(ids ...ConflictID) []*conflict.Conflict[ConflictID, ResourceID] {
	conflicts := make([]*conflict.Conflict[ConflictID, ResourceID], 0)
	for _, id := range ids {
		if existingConflict, exists := c.conflictsByID.Get(id); exists {
			conflicts = append(conflicts, existingConflict)
		}
	}

	return conflicts
}

// ConflictSets returns the ConflictSets that are associated with the given ResourceIDs.
func (c *ConflictDAG[ConflictID, ResourceID]) ConflictSets(resourceIDs ...ResourceID) []*conflict.Set[ConflictID, ResourceID] {
	conflictSets := make([]*conflict.Set[ConflictID, ResourceID], 0)
	for _, resourceID := range resourceIDs {
		conflictSets = append(conflictSets, lo.Return1(c.conflictSetsByID.GetOrCreate(resourceID, func() *conflict.Set[ConflictID, ResourceID] {
			return conflict.NewSet[ConflictID, ResourceID](resourceID)
		})))
	}

	return conflictSets
}
