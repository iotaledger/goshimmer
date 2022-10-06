package causalorder

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

// region CausalOrderer ////////////////////////////////////////////////////////////////////////////////////////////////

// CausalOrder represents an order where an Entity is ordered after its causal dependencies (parents) have been ordered.
type CausalOrder[ID epoch.IndexedID, Entity OrderedEntity[ID]] struct {
	// evictionManager contains the local manager used to orchestrate the eviction of old Entities.
	evictionManager *eviction.LockableManager[ID]

	// entityProvider contains a function that provides the Entity that belongs to a given ID.
	entityProvider func(id ID) (entity Entity, exists bool)

	// isOrdered contains a function that determines if an Entity has been ordered already.
	isOrdered func(entity Entity) (isOrdered bool)

	// orderedCallback contains a function that is called when an Entity is ordered.
	orderedCallback func(entity Entity) (err error)

	// evictionCallback contains a function that is called whenever an Entity is evicted from the CausalOrderer.
	evictionCallback func(entity Entity, reason error)

	// checkReference contains a function that checks if a reference between a child and its parents is valid.
	checkReference func(child Entity, parent Entity) (err error)

	// unorderedParentsCounter contains an in-memory storage that keeps track of the unordered parents of an Entity.
	unorderedParentsCounter *memstorage.EpochStorage[ID, uint8]

	// unorderedParentsCounterMutex contains a mutex used to synchronize access to the unorderedParentsCounter.
	unorderedParentsCounterMutex sync.Mutex

	// unorderedChildren contains an in-memory storage of the pending children of an unordered Entity.
	unorderedChildren *memstorage.EpochStorage[ID, []Entity]

	// unorderedChildrenMutex contains a mutex used to synchronize access to the unorderedChildren.
	unorderedChildrenMutex sync.Mutex

	// dagMutex contains a mutex used to synchronize access to Entities.
	dagMutex *syncutils.DAGMutex[ID]
}

// New returns a new CausalOrderer instance with the given parameters.
func New[ID epoch.IndexedID, Entity OrderedEntity[ID]](evictionManager *eviction.State[ID],
	entityProvider func(id ID) (entity Entity, exists bool),
	isOrdered func(entity Entity) (isOrdered bool),
	orderedCallback func(entity Entity) (err error),
	evictionCallback func(entity Entity, reason error),
	opts ...options.Option[CausalOrder[ID, Entity]],
) (newCausalOrder *CausalOrder[ID, Entity]) {
	return options.Apply(&CausalOrder[ID, Entity]{
		evictionManager:         evictionManager.Lockable(),
		entityProvider:          entityProvider,
		isOrdered:               isOrdered,
		orderedCallback:         orderedCallback,
		evictionCallback:        evictionCallback,
		checkReference:          checkReference[ID, Entity],
		unorderedParentsCounter: memstorage.NewEpochStorage[ID, uint8](),
		unorderedChildren:       memstorage.NewEpochStorage[ID, []Entity](),
		dagMutex:                syncutils.NewDAGMutex[ID](),
	}, opts)
}

// Queue adds the given Entity to the CausalOrderer and triggers it when it's ready.
func (c *CausalOrder[ID, Entity]) Queue(entity Entity) {
	c.evictionManager.RLock()
	defer c.evictionManager.RUnlock()

	c.triggerOrderedIfReady(entity)
}

// EvictEpoch removes all Entities that are older than the given epoch from the CausalOrderer.
func (c *CausalOrder[ID, Entity]) EvictEpoch(index epoch.Index) {
	for _, evictedEntity := range c.evictEpoch(index) {
		c.evictionCallback(evictedEntity, errors.Errorf("entity evicted from %s", index))
	}
}

// triggerOrderedIfReady triggers the ordered callback of the given Entity if it's ready.
func (c *CausalOrder[ID, Entity]) triggerOrderedIfReady(entity Entity) {
	c.dagMutex.RLock(entity.Parents()...)
	defer c.dagMutex.RUnlock(entity.Parents()...)
	c.dagMutex.Lock(entity.ID())
	defer c.dagMutex.Unlock(entity.ID())

	if c.isOrdered(entity) {
		return
	}

	if c.evictionManager.MaxEvictedEpoch() >= entity.ID().Index() {
		c.evictionCallback(entity, errors.Errorf("entity %s below max evicted epoch", entity.ID()))

		return
	}

	if !c.allParentsOrdered(entity) {
		return
	}

	c.triggerOrderedCallback(entity)
}

// allParentsOrdered returns true if all parents of the given Entity are ordered.
func (c *CausalOrder[ID, Entity]) allParentsOrdered(entity Entity) (allParentsOrdered bool) {
	pendingParents := uint8(0)
	for _, parentID := range entity.Parents() {
		parentEntity, exists := c.entityProvider(parentID)
		if !exists {
			c.evictionCallback(entity, errors.Errorf("parent %s not found", parentID))

			return
		}

		if err := c.checkReference(entity, parentEntity); err != nil {
			c.evictionCallback(entity, err)

			return
		}

		if !c.isOrdered(parentEntity) {
			pendingParents++

			c.registerUnorderedChild(parentID, entity)
		}
	}

	if pendingParents != 0 {
		c.setUnorderedParentsCounter(entity.ID(), pendingParents)
	}

	return pendingParents == 0
}

// registerUnorderedChild registers the given Entity as a child of the given parent ID.
func (c *CausalOrder[ID, Entity]) registerUnorderedChild(entityID ID, child Entity) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	unorderedChildrenStorage := c.unorderedChildren.Get(entityID.Index(), true)
	entityChildren, _ := unorderedChildrenStorage.Get(entityID)
	unorderedChildrenStorage.Set(entityID, append(entityChildren, child))

}

// setUnorderedParentsCounter sets the unordered parents counter of the given Entity to the given value.
func (c *CausalOrder[ID, Entity]) setUnorderedParentsCounter(entityID ID, unorderedParentsCount uint8) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	c.unorderedParentsCounter.Get(entityID.Index(), true).Set(entityID, unorderedParentsCount)
}

// decrementUnorderedParentsCounter decrements the unordered parents counter of the given Entity by 1 and returns the
// new value.
func (c *CausalOrder[ID, Entity]) decreaseUnorderedParentsCounter(metadata Entity) (newUnorderedParentsCounter uint8) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	unorderedParentsCounterStorage := c.unorderedParentsCounter.Get(metadata.ID().Index())
	newUnorderedParentsCounter, _ = unorderedParentsCounterStorage.Get(metadata.ID())
	if newUnorderedParentsCounter--; newUnorderedParentsCounter == 0 {
		unorderedParentsCounterStorage.Delete(metadata.ID())

		return
	}

	unorderedParentsCounterStorage.Set(metadata.ID(), newUnorderedParentsCounter)

	return
}

// popUnorderedChild pops the children of the given parent ID from the unordered children storage.
func (c *CausalOrder[ID, Entity]) popUnorderedChildren(entityID ID) (pendingChildren []Entity) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	pendingChildrenStorage := c.unorderedChildren.Get(entityID.Index())
	if pendingChildrenStorage == nil {
		return pendingChildren
	}

	pendingChildren, _ = pendingChildrenStorage.Get(entityID)

	pendingChildrenStorage.Delete(entityID)

	return pendingChildren
}

// triggerChildIfReady triggers the ordered callback of the given Entity if it's unorderedParentsCounter reaches 0
// (after decreasing it).
func (c *CausalOrder[ID, Entity]) triggerChildIfReady(child Entity) {
	c.dagMutex.Lock(child.ID())
	defer c.dagMutex.Unlock(child.ID())

	if !c.isOrdered(child) && c.decreaseUnorderedParentsCounter(child) == 0 {
		c.triggerOrderedCallback(child)
	}
}

// triggerOrderedCallback triggers the ordered callback of the given Entity and propagates .
func (c *CausalOrder[ID, Entity]) triggerOrderedCallback(entity Entity) (wasTriggered bool) {
	if err := c.orderedCallback(entity); err != nil {
		c.evictionCallback(entity, err)

		return
	}

	c.propagateOrderToChildren(entity.ID())

	return true
}

// propagateOrderToChildren propagates the order of the given Entity to its children.
func (c *CausalOrder[ID, Entity]) propagateOrderToChildren(id ID) {
	for _, child := range c.popUnorderedChildren(id) {
		currentChild := child

		event.Loop.Submit(func() {
			c.evictionManager.RLock()
			defer c.evictionManager.RUnlock()

			c.triggerChildIfReady(currentChild)
		})
	}
}

// entity returns the Entity with the given ID.
func (c *CausalOrder[ID, Entity]) entity(blockID ID) (entity Entity) {
	entity, exists := c.entityProvider(blockID)
	if !exists {
		panic(errors.Errorf("block %s does not exist", blockID))
	}

	return entity
}

// evictEpoch evicts the given Epoch from the CausalOrder and returns the evicted Entities.
func (c *CausalOrder[ID, Entity]) evictEpoch(epochIndex epoch.Index) (evictedEntities map[ID]Entity) {
	c.evictionManager.Lock()
	defer c.evictionManager.Unlock()

	evictedEntities = make(map[ID]Entity)
	c.evictEntitiesFromEpoch(epochIndex, func(id ID) {
		if _, exists := evictedEntities[id]; !exists {
			evictedEntities[id] = c.entity(id)
		}
	})

	return evictedEntities
}

// evictEntitiesFromEpoch evicts the Entities that belong to the given Epoch from the CausalOrder.
func (c *CausalOrder[ID, Entity]) evictEntitiesFromEpoch(epochIndex epoch.Index, entityCallback func(id ID)) {
	if childrenStorage := c.unorderedChildren.Get(epochIndex); childrenStorage != nil {
		childrenStorage.ForEachKey(func(id ID) bool {
			entityCallback(id)

			return true
		})
		c.unorderedChildren.EvictEpoch(epochIndex)
	}

	if unorderedParentsCountStorage := c.unorderedParentsCounter.Get(epochIndex); unorderedParentsCountStorage != nil {
		unorderedParentsCountStorage.ForEachKey(func(id ID) bool {
			entityCallback(id)

			return true
		})
		c.unorderedParentsCounter.EvictEpoch(epochIndex)
	}
}

// checkReference is the default function that checks if the given reference is valid.
func checkReference[ID epoch.IndexedID, Entity OrderedEntity[ID]](entity Entity, parent Entity) (err error) {
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithReferenceValidator is an option that sets the ReferenceValidator of the CausalOrder.
func WithReferenceValidator[ID epoch.IndexedID, Entity OrderedEntity[ID]](referenceValidator func(entity Entity, parent Entity) (err error)) options.Option[CausalOrder[ID, Entity]] {
	return func(causalOrder *CausalOrder[ID, Entity]) {
		causalOrder.checkReference = referenceValidator
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Types ////////////////////////////////////////////////////////////////////////////////////////////////////////

// OrderedEntity is an interface that represents an Entity that can be causally ordered.
type OrderedEntity[ID epoch.IndexedID] interface {
	// ID returns the ID of the Entity.
	ID() ID

	// Parents returns the causal parents of the Entity.
	Parents() []ID

	// comparable embeds the comparable interface.
	comparable
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
