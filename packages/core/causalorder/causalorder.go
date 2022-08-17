package causalorder

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

type CausalOrder[ID epoch.IndexedID, Entity OrderedEntity[ID]] struct {
	evictionManager *eviction.LockableManager
	entityProvider  func(id ID) (entity Entity, exists bool)
	isOrdered       func(entity Entity) (isOrdered bool)
	orderedCallback func(entity Entity) (err error)
	droppedCallback func(entity Entity, reason error)
	checkReference  func(child Entity, parent Entity) (err error)

	unorderedParentsCounter      *memstorage.EpochStorage[ID, uint8]
	unorderedParentsCounterMutex sync.Mutex
	unorderedChildren            *memstorage.EpochStorage[ID, []Entity]
	unorderedChildrenMutex       sync.Mutex

	dagMutex *syncutils.DAGMutex[ID]
}

func New[ID epoch.IndexedID, Entity OrderedEntity[ID]](
	evictionManager *eviction.Manager,
	entityProvider func(id ID) (entity Entity, exists bool),
	isOrdered func(entity Entity) (isOrdered bool),
	orderedCallback func(entity Entity) (err error),
	droppedCallback func(entity Entity, reason error),
	opts ...options.Option[CausalOrder[ID, Entity]],
) (newCausalOrder *CausalOrder[ID, Entity]) {
	newCausalOrder = options.Apply(&CausalOrder[ID, Entity]{
		evictionManager:         evictionManager.Lockable(),
		entityProvider:          entityProvider,
		isOrdered:               isOrdered,
		orderedCallback:         orderedCallback,
		droppedCallback:         droppedCallback,
		checkReference:          func(entity Entity, parent Entity) (err error) { return nil },
		unorderedParentsCounter: memstorage.NewEpochStorage[ID, uint8](),
		unorderedChildren:       memstorage.NewEpochStorage[ID, []Entity](),
		dagMutex:                syncutils.NewDAGMutex[ID](),
	}, opts)

	return newCausalOrder
}

func (c *CausalOrder[ID, Entity]) Queue(entity Entity) {
	c.evictionManager.RLock()
	defer c.evictionManager.RUnlock()

	c.triggerOrderedIfReady(entity)
}

func (c *CausalOrder[ID, Entity]) EvictEpoch(index epoch.Index) {
	for _, evictedEntity := range c.evictEntities(index) {
		c.droppedCallback(evictedEntity, errors.Errorf("entity evicted from %s", index))
	}
}

func (c *CausalOrder[ID, Entity]) triggerOrderedIfReady(entity Entity) {
	c.dagMutex.RLock(entity.Parents()...)
	defer c.dagMutex.RUnlock(entity.Parents()...)
	c.dagMutex.Lock(entity.ID())
	defer c.dagMutex.Unlock(entity.ID())

	if c.isOrdered(entity) || c.evictionManager.MaxDroppedEpoch() >= entity.ID().Index() || !c.allParentsOrdered(entity) {
		return
	}

	c.triggerOrderedCallback(entity)
}

func (c *CausalOrder[ID, Entity]) allParentsOrdered(entity Entity) (allParentsOrdered bool) {
	pendingParents := uint8(0)
	for _, parentID := range entity.Parents() {
		parentEntity, exists := c.entityProvider(parentID)
		if !exists {
			c.droppedCallback(entity, errors.Errorf("parent %s not found", parentID))

			return
		}

		if err := c.checkReference(entity, parentEntity); err != nil {
			c.droppedCallback(entity, err)

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

func (c *CausalOrder[ID, Entity]) registerUnorderedChild(entityID ID, child Entity) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	unorderedChildrenStorage := c.unorderedChildren.Get(entityID.Index(), true)
	entityChildren, _ := unorderedChildrenStorage.Get(entityID)
	unorderedChildrenStorage.Set(entityID, append(entityChildren, child))

}

func (c *CausalOrder[ID, Entity]) setUnorderedParentsCounter(entityID ID, unorderedParentsCount uint8) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	c.unorderedParentsCounter.Get(entityID.Index(), true).Set(entityID, unorderedParentsCount)
}

func (c *CausalOrder[ID, Entity]) decreaseUnorderedParentsCounter(metadata Entity) (newUnorderedParentsCounter uint8) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	unorderedParentsCounterStorage := c.unorderedParentsCounter.Get(metadata.ID().Index())
	if unorderedParentsCounterStorage == nil {
		panic(fmt.Sprintf("unordered parents counter epoch not found for %s", metadata.ID()))
	}

	newUnorderedParentsCounter, exists := unorderedParentsCounterStorage.Get(metadata.ID())
	if !exists {
		panic(fmt.Sprintf("unordered parents counter not found for %s", metadata.ID()))
	}

	if newUnorderedParentsCounter--; newUnorderedParentsCounter == 0 {
		unorderedParentsCounterStorage.Delete(metadata.ID())

		return
	}

	unorderedParentsCounterStorage.Set(metadata.ID(), newUnorderedParentsCounter)

	return
}

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

func (c *CausalOrder[ID, Entity]) triggerChildIfReady(child Entity) {
	c.dagMutex.Lock(child.ID())
	defer c.dagMutex.Unlock(child.ID())

	if !c.isOrdered(child) && c.decreaseUnorderedParentsCounter(child) == 0 {
		c.triggerOrderedCallback(child)
	}
}

func (c *CausalOrder[ID, Entity]) triggerOrderedCallback(entity Entity) (wasTriggered bool) {
	if err := c.orderedCallback(entity); err != nil {
		c.droppedCallback(entity, err)

		return
	}

	c.propagateOrderToChildren(entity.ID())

	return true
}

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

func (c *CausalOrder[ID, Entity]) entity(blockID ID) (entity Entity) {
	entity, exists := c.entityProvider(blockID)
	if !exists {
		panic(errors.Errorf("block %s does not exist", blockID))
	}

	return entity
}

func (c *CausalOrder[ID, Entity]) evictEntities(epochIndex epoch.Index) (evictedEntities map[ID]Entity) {
	c.evictionManager.Lock()
	defer c.evictionManager.Unlock()

	evictedEntities = make(map[ID]Entity)
	c.dropEntitiesFromEpoch(epochIndex, func(id ID) {
		if _, exists := evictedEntities[id]; !exists {
			evictedEntities[id] = c.entity(id)
		}
	})

	return evictedEntities
}

func (c *CausalOrder[ID, Entity]) dropEntitiesFromEpoch(epochIndex epoch.Index, entityCallback func(id ID)) {
	if childrenStorage := c.unorderedChildren.Get(epochIndex); childrenStorage != nil {
		childrenStorage.ForEachKey(func(id ID) bool {
			entityCallback(id)

			return true
		})
		c.unorderedChildren.Drop(epochIndex)
	}

	if unorderedParentsCountStorage := c.unorderedParentsCounter.Get(epochIndex); unorderedParentsCountStorage != nil {
		unorderedParentsCountStorage.ForEachKey(func(id ID) bool {
			entityCallback(id)

			return true
		})
		c.unorderedParentsCounter.Drop(epochIndex)
	}
}
