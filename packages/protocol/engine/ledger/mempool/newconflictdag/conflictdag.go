package newconflictdag

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
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
func (c *ConflictDAG[ConflictID, ResourceID]) CreateConflict(id ConflictID, parentIDs []ConflictID, resourceIDs []ResourceID) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	newConflict, newConflictCreated := c.conflictsByID.GetOrCreate(id, func() *conflict.Conflict[ConflictID, ResourceID] {
		return conflict.New[ConflictID, ResourceID](id, c.Conflicts(parentIDs...), c.ConflictSets(resourceIDs...), nil, c.pendingTasks)
	})

	if newConflictCreated {
		c.ConflictCreated.Trigger(newConflict)
	}

	return newConflictCreated
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
