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

// CreateConflict creates a new Conflict in the ConflictDAG and returns true if the Conflict was created.
func (b *ConflictDAG[ConflictIDType, ResourceIDType]) CreateConflict(id ConflictIDType, parentIDs *set.AdvancedSet[ConflictIDType], conflictingResourceIDs *set.AdvancedSet[ResourceIDType]) (created bool) {
	b.mutex.Lock()

	parents := set.NewAdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]()
	for it := parentIDs.Iterator(); it.HasNext(); {
		parentID := it.Next()
		parent, exists := b.conflicts.Get(parentID)
		if !exists {
			// TODO: what do we do? does it mean the parent has been evicted already?
		}
		parents.Add(parent)
	}

	conflict, created := b.conflicts.RetrieveOrCreate(id, func() (newConflict *Conflict[ConflictIDType, ResourceIDType]) {
		newConflict = NewConflict[ConflictIDType, ResourceIDType](id, parents, set.NewAdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]())

		for it := conflictingResourceIDs.Iterator(); it.HasNext(); {
			conflictSetID := it.Next()

			conflictSet, _ := b.conflictSets.RetrieveOrCreate(conflictSetID, func() *ConflictSet[ConflictIDType, ResourceIDType] {
				return NewConflictSet[ConflictIDType, ResourceIDType](conflictSetID)
			})
			conflictSet.AddMember(newConflict)
			newConflict.addConflictSet(conflictSet)
		}

		// create parent references to newly created conflict
		for it := parents.Iterator(); it.HasNext(); {
			parent := it.Next()
			parent.addChild(newConflict)
		}

		if b.anyParentRejected(newConflict) || b.anyConflictingConflictAccepted(conflict) {
			newConflict.setConfirmationState(confirmation.Rejected)
		}

		return
	})

	b.mutex.Unlock()

	if created {
		b.Events.ConflictCreated.Trigger(conflict)
	}

	return created
}

// TODO: should we add this to the conflict model itself?
// anyParentRejected checks if any of a Conflicts parents is Rejected.
func (b *ConflictDAG[ConflictID, ConflictingResourceID]) anyParentRejected(conflict *Conflict[ConflictID, ConflictingResourceID]) (rejected bool) {
	for it := conflict.Parents().Iterator(); it.HasNext(); {
		if it.Next().ConfirmationState().IsRejected() {
			return true
		}
	}

	return false
}
