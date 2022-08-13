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

func (c *CausalOrder[ID, Entity]) Queue(entity Entity) (isOrdered bool) {
	c.pruningMutex.RLock()
	defer c.pruningMutex.RUnlock()

	if entity.ID().Index() <= c.maxDroppedEpoch {
		c.triggerDroppedCallback(entity, errors.Errorf("entity %s is too old", entity.ID()))

		return false
	}

	return c.triggerOrderedIfReady(entity)
}

func (c *CausalOrder[ID, Entity]) Prune(epochIndex epoch.Index) {
	for _, droppedEntity := range c.dropEntities(epochIndex) {
		c.triggerDroppedCallback(droppedEntity, errors.Errorf("entity %s pruned", droppedEntity.ID()))
	}
}

func (c *CausalOrder[ID, Entity]) triggerOrderedIfReady(entity Entity) (wasOrdered bool) {
	c.lockEntity(entity)
	defer c.unlockEntity(entity)

	if parentsOrdered, err := c.checkParents(entity); !parentsOrdered {
		if err != nil {
			c.triggerDroppedCallback(entity, err)
		}

		return
	}

	return c.triggerOrderedCallback(entity)
}

func (c *CausalOrder[ID, Entity]) checkParents(entity Entity) (parentsOrdered bool, err error) {
	pendingParents := uint8(0)
	for _, parentID := range entity.Parents() {
		parentEntity := c.entity(parentID)

		if !c.isReferenceValid(entity, parentEntity) {
			return false, errors.Errorf("entity %s has an invalid parent", entity.ID())
		}

		if !c.isOrdered(parentEntity) {
			pendingParents++

			c.registerUnorderedChild(parentID, entity)
		}
	}

	c.setUnorderedParentsCounter(entity.ID(), pendingParents)

	return pendingParents == 0, nil
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
		c.triggerOrderedCallback(child)
	}
}

func (c *CausalOrder[ID, Entity]) triggerOrderedCallback(entity Entity) (wasTriggered bool) {
	if err := c.orderedCallback(entity); err != nil {
		c.triggerDroppedCallback(entity, err)

		return
	}

	for _, child := range c.popUnorderedChildren(entity.ID()) {
		currentChild := child

		event.Loop.Submit(func() { c.triggerChildIfReady(currentChild) })
	}

	return true
}

func (c *CausalOrder[ID, Entity]) triggerDroppedCallback(entity Entity, _ error) {
	c.Events.Drop.Trigger(entity)
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
