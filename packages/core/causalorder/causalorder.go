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

type CausalOrder[ID epoch.IndexedID, Entity OrderedEntity[ID]] struct {
	Events *Events[ID, Entity]

	entityProvider   func(id ID) (entity Entity, exists bool)
	isOrdered        func(entity Entity) (isOrdered bool)
	setOrdered       func(entity Entity, value bool) (wasUpdated bool)
	genesisBlock     func(id ID) (isGenesis Entity)
	isReferenceValid func(child Entity, parent Entity) (isValid bool)

	unorderedParentsCounter      *memstorage.EpochStorage[ID, uint8]
	unorderedParentsCounterMutex sync.Mutex
	unorderedChildren            *memstorage.EpochStorage[ID, []Entity]
	unorderedChildrenMutex       sync.Mutex

	maxDroppedEpoch epoch.Index
	pruningMutex    sync.RWMutex
}

func New[ID epoch.IndexedID, Entity OrderedEntity[ID]](
	entityProvider func(ID) (entity Entity, exists bool),
	isOrdered func(Entity) bool,
	setOrdered func(Entity, bool) bool,
	genesisEntityProvider func(ID) Entity,
	opts ...options.Option[CausalOrder[ID, Entity]],
) *CausalOrder[ID, Entity] {
	return options.Apply(&CausalOrder[ID, Entity]{
		Events: newEvents[ID, Entity](),

		entityProvider:          entityProvider,
		isOrdered:               isOrdered,
		setOrdered:              setOrdered,
		genesisBlock:            genesisEntityProvider,
		unorderedParentsCounter: memstorage.NewEpochStorage[ID, uint8](),
		unorderedChildren:       memstorage.NewEpochStorage[ID, []Entity](),

		isReferenceValid: func(entity Entity, parent Entity) bool {
			return true
		},
	}, opts)
}

func (c *CausalOrder[I, Entity]) Queue(entity Entity) (ordered bool) {
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

func (c *CausalOrder[I, Entity]) Prune(epochIndex epoch.Index) {
	for _, droppedEntity := range c.dropEntities(epochIndex) {
		c.Events.Drop.Trigger(droppedEntity)
	}
}

func (c *CausalOrder[I, Entity]) dropEntities(epochIndex epoch.Index) (droppedEntities map[I]Entity) {
	c.pruningMutex.Lock()
	defer c.pruningMutex.Unlock()

	if epochIndex <= c.maxDroppedEpoch {
		return
	}

	droppedEntities = make(map[I]Entity)
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

func (c *CausalOrder[I, Entity]) dropEntitiesFromEpoch(epochIndex epoch.Index, entityCallback func(id I)) {
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

func (c *CausalOrder[I, Entity]) wasOrdered(entity Entity) (wasOrdered bool) {
	c.lockEntity(entity)
	defer c.unlockEntity(entity)

	if updatedStatus := c.updateOrderStatus(entity); updatedStatus == Invalid {
		c.Events.Drop.Trigger(entity)
	} else if wasOrdered = updatedStatus == Ordered; wasOrdered {
		c.Events.Emit.Trigger(entity)
	}

	return
}

func (c *CausalOrder[I, Entity]) updateOrderStatus(entity Entity) (updateType UpdateType) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	pendingParentsCount, anyParentInvalid := c.countPendingParents(entity)
	if anyParentInvalid {
		return Invalid
	}

	c.unorderedParentsCounter.Get(entity.ID().Index(), true).Set(entity.ID(), pendingParentsCount)
	if pendingParentsCount != 0 || !c.setOrdered(entity, true) {
		return Unchanged
	}

	return Ordered
}

func (c *CausalOrder[ID, Entity]) isGenesis(id ID) bool {
	var emptyEntity Entity

	return c.genesisBlock(id) != emptyEntity
}

func (c *CausalOrder[I, Entity]) countPendingParents(entity Entity) (pendingParents uint8, areParentsInvalid bool) {
	for _, parentID := range entity.Parents() {
		parentEntity := c.entity(parentID)

		if !c.isReferenceValid(entity, parentEntity) || (!c.isGenesis(parentID) && parentID.Index() <= c.maxDroppedEpoch) {
			return pendingParents, true
		}

		if !c.isOrdered(parentEntity) {
			pendingParents++

			c.registerUnorderedChild(parentID, entity)
		}
	}

	return pendingParents, false
}

func (c *CausalOrder[I, Entity]) registerUnorderedChild(entityID I, child Entity) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	unorderedChildrenStorage := c.unorderedChildren.Get(entityID.Index(), true)

	entityChildren, _ := unorderedChildrenStorage.Get(entityID)
	unorderedChildrenStorage.Set(entityID, append(entityChildren, child))

}

func (c *CausalOrder[I, Entity]) popUnorderedChildren(entityID I) (pendingChildren []Entity) {
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

func (c *CausalOrder[I, Entity]) propagateCausalOrder(metadata Entity) {
	for childWalker := walker.New[I](true).Push(metadata.ID()); childWalker.HasNext(); {
		for _, child := range c.popUnorderedChildren(childWalker.Next()) {
			if c.triggerChildIfReady(child) {
				childWalker.Push(child.ID())
			}
		}
	}
}

func (c *CausalOrder[I, Entity]) triggerChildIfReady(metadata Entity) (eventTriggered bool) {
	metadata.Lock()
	defer metadata.Unlock()

	if c.decreaseUnorderedParentsCounter(metadata) != 0 || !c.setOrdered(metadata, true) {
		return false
	}

	c.Events.Emit.Trigger(metadata)

	return true
}

func (c *CausalOrder[I, Entity]) decreaseUnorderedParentsCounter(metadata Entity) (unorderedParentsCounter uint8) {
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

func (c *CausalOrder[I, Entity]) lockEntity(entity Entity) {
	for _, parentID := range entity.Parents() {
		c.entity(parentID).RLock()
	}

	entity.Lock()
}

func (c *CausalOrder[I, Entity]) unlockEntity(metadata Entity) {
	for _, parentID := range metadata.Parents() {
		c.entity(parentID).RUnlock()
	}

	metadata.Unlock()
}

func (c *CausalOrder[I, Entity]) entity(blockID I) (entity Entity) {
	entity, exists := c.entityProvider(blockID)
	if !exists {
		panic(errors.Errorf("block %s does not exist", blockID))
	}

	return entity
}
