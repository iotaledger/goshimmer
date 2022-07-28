package causalorder

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

type CausalOrder[ID epoch.IndexedID, Entity EntityInterface[ID]] struct {
	Events *Events[ID, Entity]

	entityProvider   func(id ID) (entity Entity, exists bool)
	isOrdered        func(entity Entity) (isOrdered bool)
	setOrdered       func(entity Entity, value bool) (wasUpdated bool)
	isGenesis        func(id ID) (isGenesis bool)
	isReferenceValid func(child Entity, parent Entity) (isValid bool)

	unorderedParentsCounter      *memstorage.EpochStorage[ID, uint8]
	unorderedParentsCounterMutex sync.Mutex
	unorderedChildren            *memstorage.EpochStorage[ID, []Entity]
	unorderedChildrenMutex       sync.Mutex

	maxDroppedEpoch epoch.Index
	pruningMutex    sync.RWMutex
}

func New[ID epoch.IndexedID, EntityT EntityInterface[ID]](
	entityProvider func(ID) (entity EntityT, exists bool),
	isOrdered func(EntityT) bool,
	setOrdered func(EntityT, bool) bool,
	isGenesis func(ID) bool,
	opts ...options.Option[CausalOrder[ID, EntityT]],
) *CausalOrder[ID, EntityT] {
	return options.Apply(&CausalOrder[ID, EntityT]{
		Events: newEvents[ID, EntityT](),

		entityProvider:          entityProvider,
		isOrdered:               isOrdered,
		setOrdered:              setOrdered,
		isGenesis:               isGenesis,
		unorderedParentsCounter: memstorage.NewEpochStorage[ID, uint8](),
		unorderedChildren:       memstorage.NewEpochStorage[ID, []EntityT](),

		isReferenceValid: func(entity EntityT, parent EntityT) bool {
			return true
		},
	}, opts)
}

func (c *CausalOrder[I, EntityType]) Queue(entity EntityType) (ordered bool) {
	c.pruningMutex.RLock()
	defer c.pruningMutex.RUnlock()

	if entity.ID().Index() <= c.maxDroppedEpoch {
		c.Events.Drop.Trigger(entity)
		return
	}

	if ordered = c.wasOrdered(entity); ordered {
		c.propagateCausalOrder(entity)
	}

	return ordered
}

func (c *CausalOrder[I, EntityType]) Prune(epochIndex epoch.Index) {
	for _, droppedEntity := range c.dropEntities(epochIndex) {
		c.Events.Drop.Trigger(droppedEntity)
	}
}

func (c *CausalOrder[I, EntityType]) dropEntities(epochIndex epoch.Index) (droppedEntities map[I]EntityType) {
	c.pruningMutex.Lock()
	defer c.pruningMutex.Unlock()

	if epochIndex <= c.maxDroppedEpoch {
		return
	}

	droppedEntities = make(map[I]EntityType)
	for currentEpoch := c.maxDroppedEpoch + 1; currentEpoch <= epochIndex; currentEpoch++ {
		c.dropEntitiesFromEpoch(currentEpoch, func(id I) {
			if _, exists := droppedEntities[id]; !exists {
				droppedEntities[id] = c.entity(id)
			}
		})
	}
	c.maxDroppedEpoch = epochIndex

	return droppedEntities
}

func (c *CausalOrder[I, EntityType]) dropEntitiesFromEpoch(epochIndex epoch.Index, entityCallback func(id I)) {
	c.unorderedChildren.Get(epochIndex).ForEachKey(func(id I) bool {
		entityCallback(id)

		return true
	})
	c.unorderedChildren.Drop(epochIndex)

	c.unorderedParentsCounter.Get(epochIndex).ForEachKey(func(id I) bool {
		entityCallback(id)

		return true
	})
	c.unorderedParentsCounter.Drop(epochIndex)
}

func (c *CausalOrder[I, EntityType]) wasOrdered(entity EntityType) (wasOrdered bool) {
	c.lockEntity(entity)
	defer c.unlockEntity(entity)

	if updatedStatus := c.updateStatus(entity); updatedStatus == Invalid {
		c.Events.Drop.Trigger(entity)
	} else if wasOrdered = updatedStatus == Ordered; wasOrdered {
		c.Events.Emit.Trigger(entity)
	}

	return
}

func (c *CausalOrder[I, EntityType]) updateStatus(entity EntityType) (orderStatus StatusUpdate) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	unorderedParents, isInvalid := c.checkParents(entity)
	if isInvalid {
		return Invalid
	}

	c.unorderedParentsCounter.Get(entity.ID().Index(), true).Set(entity.ID(), unorderedParents)
	if unorderedParents != 0 || !c.setOrdered(entity, true) {
		return Unchanged
	}

	return Ordered
}

func (c *CausalOrder[I, EntityType]) checkParents(entity EntityType) (unorderedParentsCount uint8, anyParentInvalid bool) {
	for _, parentID := range entity.ParentIDs() {
		parentEntity := c.entity(parentID)

		if !c.isReferenceValid(entity, parentEntity) || (!c.isGenesis(parentID) && parentID.Index() <= c.maxDroppedEpoch) {
			return unorderedParentsCount, true
		}

		if !c.isOrdered(parentEntity) {
			unorderedParentsCount++

			c.registerUnorderedChild(parentID, entity)
		}
	}

	return unorderedParentsCount, false
}

func (c *CausalOrder[I, EntityType]) registerUnorderedChild(entityID I, child EntityType) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	unorderedChildrenStorage := c.unorderedChildren.Get(entityID.Index(), true)

	entityChildren, _ := unorderedChildrenStorage.Get(entityID)
	unorderedChildrenStorage.Set(entityID, append(entityChildren, child))

}

func (c *CausalOrder[I, EntityType]) popUnorderedChildren(entityID I) (pendingChildren []EntityType) {
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

func (c *CausalOrder[I, EntityType]) propagateCausalOrder(metadata EntityType) {
	for childWalker := walker.New[I](true).Push(metadata.ID()); childWalker.HasNext(); {
		for _, child := range c.popUnorderedChildren(childWalker.Next()) {
			if c.triggerChildIfReady(child) {
				childWalker.Push(child.ID())
			}
		}
	}
}

func (c *CausalOrder[I, EntityType]) triggerChildIfReady(metadata EntityType) (eventTriggered bool) {
	metadata.Lock()
	defer metadata.Unlock()

	if c.decreaseUnorderedParentsCounter(metadata) != 0 || !c.setOrdered(metadata, true) {
		return false
	}

	c.Events.Emit.Trigger(metadata)

	return true
}

func (c *CausalOrder[I, EntityType]) decreaseUnorderedParentsCounter(metadata EntityType) (unorderedParentsCounter uint8) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	unorderedParentsCounterStorage := c.unorderedParentsCounter.Get(metadata.ID().Index())
	if unorderedParentsCounterStorage == nil {
		// entity was already dropped and can never be considered as ordered
		panic(fmt.Sprintf("unordered parents counter epoch not found for %s", metadata.ID()))
	}

	unorderedParentsCounter, exists := unorderedParentsCounterStorage.Get(metadata.ID())
	if !exists {
		panic(fmt.Sprintf("unordered parents counter not found for %s", metadata.ID()))
	}
	unorderedParentsCounter--
	unorderedParentsCounterStorage.Set(metadata.ID(), unorderedParentsCounter)

	return
}

func (c *CausalOrder[I, EntityType]) lockEntity(entity EntityType) {
	for _, parentID := range entity.ParentIDs() {
		c.entity(parentID).RLock()
	}

	entity.Lock()
}

func (c *CausalOrder[I, EntityType]) unlockEntity(metadata EntityType) {
	for _, parentID := range metadata.ParentIDs() {
		c.entity(parentID).RUnlock()
	}

	metadata.Unlock()
}

func (c *CausalOrder[I, EntityType]) entity(blockID I) (entity EntityType) {
	entity, exists := c.entityProvider(blockID)
	if !exists {
		panic(errors.Errorf("block %s does not exist", blockID))
	}

	return entity
}
