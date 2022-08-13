package causalorder

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

type CausalOrder[ID epoch.IndexedID, Entity OrderedEntity[ID]] struct {
	Events *Events[ID, Entity]

	entityProvider   func(id ID) (entity Entity, exists bool)
	isOrdered        func(entity Entity) (isOrdered bool)
	orderedCallback  func(entity Entity) (err error)
	isReferenceValid func(child Entity, parent Entity) (isValid bool)

	unorderedParentsCounter      *memstorage.EpochStorage[ID, uint8]
	unorderedParentsCounterMutex sync.Mutex
	unorderedChildren            *memstorage.EpochStorage[ID, []Entity]
	unorderedChildrenMutex       sync.Mutex

	maxDroppedEpoch epoch.Index
	pruningMutex    sync.RWMutex
	dagMutex        *syncutils.DAGMutex[ID]
}

func New[ID epoch.IndexedID, Entity OrderedEntity[ID]](
	entityProvider func(ID) (entity Entity, exists bool),
	isOrdered func(entity Entity) (isOrdered bool),
	orderedCallback func(entity Entity) (err error),
	opts ...options.Option[CausalOrder[ID, Entity]],
) *CausalOrder[ID, Entity] {
	return options.Apply(&CausalOrder[ID, Entity]{
		Events: newEvents[ID, Entity](),

		entityProvider:          entityProvider,
		isOrdered:               isOrdered,
		orderedCallback:         orderedCallback,
		isReferenceValid:        func(entity Entity, parent Entity) bool { return true },
		unorderedParentsCounter: memstorage.NewEpochStorage[ID, uint8](),
		unorderedChildren:       memstorage.NewEpochStorage[ID, []Entity](),
		maxDroppedEpoch:         -1,
		dagMutex:                syncutils.NewDAGMutex[ID](),
	}, opts)
}

func (c *CausalOrder[ID, Entity]) Queue(entity Entity) (ordered bool) {
	c.pruningMutex.RLock()
	defer c.pruningMutex.RUnlock()

	if entity.ID().Index() <= c.maxDroppedEpoch {
		c.Events.Drop.Trigger(entity)
		return
	}

	c.triggerOrderedIfReady(entity)

	return ordered
}

func (c *CausalOrder[ID, Entity]) Prune(epochIndex epoch.Index) {
	for _, droppedEntity := range c.dropEntities(epochIndex) {
		c.Events.Drop.Trigger(droppedEntity)
	}
}

func (c *CausalOrder[ID, Entity]) dropEntities(epochIndex epoch.Index) (droppedEntities map[ID]Entity) {
	c.pruningMutex.Lock()
	defer c.pruningMutex.Unlock()

	if epochIndex <= c.maxDroppedEpoch {
		return
	}

	droppedEntities = make(map[ID]Entity)
	for c.maxDroppedEpoch < epochIndex {
		c.maxDroppedEpoch++
		c.dropEntitiesFromEpoch(c.maxDroppedEpoch, func(id ID) {
			if _, exists := droppedEntities[id]; !exists {
				droppedEntities[id] = c.entity(id)
			}
		})
	}

	return droppedEntities
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

func (c *CausalOrder[ID, Entity]) triggerOrderedIfReady(entity Entity) {
	c.lockEntity(entity)
	defer c.unlockEntity(entity)

	newUnorderedParentsCount, err := c.updateUnorderedParents(entity)
	if err != nil {
		fmt.Println("DROP", entity)
		c.Events.Drop.Trigger(entity)
	}

	if newUnorderedParentsCount != 0 {
		return
	}

	if err = c.triggerOrderedCallback(entity); err != nil {
		c.Events.Drop.Trigger(entity)
	}
}

func (c *CausalOrder[ID, Entity]) updateUnorderedParents(entity Entity) (newPendingParentsCount uint8, err error) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	newPendingParentsCount, anyParentInvalid := c.countPendingParents(entity)
	if anyParentInvalid {
		return 1, errors.Errorf("entity %s has an invalid parent", entity.ID())
	}
	c.unorderedParentsCounter.Get(entity.ID().Index(), true).Set(entity.ID(), newPendingParentsCount)

	return
}

func (c *CausalOrder[ID, Entity]) countPendingParents(entity Entity) (pendingParents uint8, areParentsInvalid bool) {
	for _, parentID := range entity.Parents() {
		parentEntity, exists := c.entityProvider(parentID)
		if !exists || !c.isReferenceValid(entity, parentEntity) {
			return pendingParents, true
		}

		if !c.isOrdered(parentEntity) {
			pendingParents++

			c.registerUnorderedChild(parentID, entity)
		}
	}

	return pendingParents, false
}

func (c *CausalOrder[ID, Entity]) registerUnorderedChild(entityID ID, child Entity) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	unorderedChildrenStorage := c.unorderedChildren.Get(entityID.Index(), true)

	entityChildren, _ := unorderedChildrenStorage.Get(entityID)
	unorderedChildrenStorage.Set(entityID, append(entityChildren, child))

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

	if c.decreaseUnorderedParentsCounter(child) == 0 {
		if err := c.triggerOrderedCallback(child); err != nil {
			c.Events.Drop.Trigger(child)
		}
	}
}

func (c *CausalOrder[ID, Entity]) triggerOrderedCallback(entity Entity) (err error) {
	if err = c.orderedCallback(entity); err != nil {
		return
	}

	for _, child := range c.popUnorderedChildren(entity.ID()) {
		currentChild := child

		event.Loop.Submit(func() { c.triggerChildIfReady(currentChild) })
	}

	return
}

func (c *CausalOrder[ID, Entity]) decreaseUnorderedParentsCounter(metadata Entity) (unorderedParentsCounter uint8) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	unorderedParentsCounterStorage := c.unorderedParentsCounter.Get(metadata.ID().Index())
	if unorderedParentsCounterStorage == nil {
		panic(fmt.Sprintf("unordered parents counter epoch not found for %s", metadata.ID()))
	}

	unorderedParentsCounter, exists := unorderedParentsCounterStorage.Get(metadata.ID())
	if !exists {
		panic(fmt.Sprintf("unordered parents counter not found for %s", metadata.ID()))
	}
	unorderedParentsCounter--
	if unorderedParentsCounter == 0 {
		unorderedParentsCounterStorage.Delete(metadata.ID())
		return
	}
	unorderedParentsCounterStorage.Set(metadata.ID(), unorderedParentsCounter)

	return
}

func (c *CausalOrder[ID, Entity]) lockEntity(entity Entity) {
	c.dagMutex.RLock(entity.Parents()...)
	c.dagMutex.Lock(entity.ID())
}

func (c *CausalOrder[ID, Entity]) unlockEntity(entity Entity) {
	c.dagMutex.Unlock(entity.ID())
	c.dagMutex.RUnlock(entity.Parents()...)
}

func (c *CausalOrder[ID, Entity]) entity(blockID ID) (entity Entity) {
	entity, exists := c.entityProvider(blockID)
	if !exists {
		panic(errors.Errorf("block %s does not exist", blockID))
	}

	return entity
}
