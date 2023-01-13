package conflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
)

// ConflictDAG represents a generic DAG that is able to model causal dependencies between conflicts that try to access a
// shared set of resources.
type ConflictDAG[ConflictIDType, ResourceIDType comparable] struct {
	// Events contains the Events of the ConflictDAG.
	Events *Events[ConflictIDType, ResourceIDType]

	// EvictionState contains information about the current eviction state.
	EvictionState *eviction.State

	conflicts    *memstorage.Storage[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]]
	conflictSets *memstorage.Storage[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]]

	// evictionMutex is a mutex that is used to synchronize the eviction of elements from the ConflictDAG.
	// TODO: use evictionMutex sync.RWMutex

	// TODO: is this mutex needed?
	// mutex RWMutex is a mutex that prevents that two processes simultaneously update the ConfirmationState.
	mutex sync.RWMutex
}

// New is the constructor for the BlockDAG and creates a new BlockDAG instance.
func New[ConflictIDType, ResourceIDType comparable](evictionState *eviction.State, opts ...options.Option[ConflictDAG[ConflictIDType, ResourceIDType]]) (c *ConflictDAG[ConflictIDType, ResourceIDType]) {
	return options.Apply(&ConflictDAG[ConflictIDType, ResourceIDType]{
		Events:        NewEvents[ConflictIDType, ResourceIDType](),
		EvictionState: evictionState,
		conflicts:     memstorage.New[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]](),
		conflictSets:  memstorage.New[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]](),
	}, opts, func(b *ConflictDAG[ConflictIDType, ResourceIDType]) {

		// TODO: evictionState.Events.EpochEvicted.Hook(event.NewClosure(b.evictEpoch))
	})
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) Conflict(conflictID ConflictIDType) (conflict *Conflict[ConflictIDType, ResourceIDType], exists bool) {
	return c.conflicts.Get(conflictID)
}

func (c *ConflictDAG[ConflictIDType, ResourceIDType]) ConflictSet(resourceID ResourceIDType) (conflictSet *ConflictSet[ConflictIDType, ResourceIDType], exists bool) {
	return c.conflictSets.Get(resourceID)
}

// CreateConflict creates a new Conflict in the ConflictDAG and returns true if the Conflict was created.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) CreateConflict(id ConflictIDType, parentIDs *set.AdvancedSet[ConflictIDType], conflictingResourceIDs *set.AdvancedSet[ResourceIDType]) (created bool) {
	c.mutex.Lock()

	conflict, created := c.conflicts.RetrieveOrCreate(id, func() (newConflict *Conflict[ConflictIDType, ResourceIDType]) {
		newConflict = NewConflict[ConflictIDType, ResourceIDType](id, parentIDs, set.NewAdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]())

		c.registerConflictWithConflictSet(newConflict, conflictingResourceIDs)

		// create parent references to newly created conflict
		for it := parentIDs.Iterator(); it.HasNext(); {
			parentID := it.Next()
			parent, exists := c.conflicts.Get(parentID)
			if !exists {
				// TODO: what do we do? does it mean the parent has been evicted already?
				continue
			}
			parent.addChild(newConflict)
		}

		if c.anyParentRejected(newConflict) || c.anyConflictingConflictAccepted(newConflict) {
			newConflict.setConfirmationState(confirmation.Rejected)
		}

		return
	})

	c.mutex.Unlock()

	if created {
		c.Events.ConflictCreated.Trigger(conflict)
	}

	return created
}

// UpdateConflictingResources adds the Conflict to the given ConflictSets - it returns true if the conflict membership was modified during this operation.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) UpdateConflictingResources(id ConflictIDType, conflictingResourceIDs *set.AdvancedSet[ResourceIDType]) (updated bool) {
	conflict, exists := c.conflicts.Get(id)
	if !exists {
		return false
	}

	c.mutex.RLock()
	updated = c.registerConflictWithConflictSet(conflict, conflictingResourceIDs)
	c.mutex.RUnlock()

	if updated {
		c.Events.ConflictUpdated.Trigger(conflict)
	}

	return updated
}

// anyParentRejected checks if any of a Conflicts parents is Rejected.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) anyParentRejected(conflict *Conflict[ConflictIDType, ResourceIDType]) (rejected bool) {
	for it := conflict.Parents().Iterator(); it.HasNext(); {
		parent, exists := c.conflicts.Get(it.Next())
		if exists && parent.ConfirmationState().IsRejected() {
			return true
		}
	}

	return false
}

// anyConflictingConflictAccepted checks if any conflicting Conflict is Accepted/Confirmed.
func (c *ConflictDAG[ConflictIDType, ResourceIDType]) anyConflictingConflictAccepted(conflict *Conflict[ConflictIDType, ResourceIDType]) (anyAccepted bool) {
	conflict.forEachConflictingConflictID(func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType]) bool {
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
