package causalorder

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

type CausalOrder[IDType ID, EntityType Entity[IDType]] struct {
	Emit *event.Event[EntityType]
	Drop *event.Event[EntityType]

	entityProvider         EntityProvider[IDType, EntityType]
	isOrdered              func(EntityType) bool
	setOrdered             func(EntityType, bool) bool
	isGenesis              func(IDType) bool
	optsReferenceValidator func(entity EntityType, parent EntityType) bool

	unorderedParentsCounter      *memstorage.EpochStorage[IDType, uint8]
	unorderedParentsCounterMutex sync.Mutex
	unorderedChildren            *memstorage.EpochStorage[IDType, []EntityType]
	unorderedChildrenMutex       sync.Mutex

	maxDroppedEpoch epoch.Index
	pruningMutex    sync.RWMutex
}

func New[IDType ID, EntityType Entity[IDType]](
	entityProvider EntityProvider[IDType, EntityType],
	isOrdered func(EntityType) bool,
	setOrdered func(EntityType, bool) bool,
	isGenesis func(IDType) bool,
	opts ...options.Option[CausalOrder[IDType, EntityType]],
) *CausalOrder[IDType, EntityType] {
	return options.Apply(&CausalOrder[IDType, EntityType]{
		Emit: event.New[EntityType](),
		Drop: event.New[EntityType](),

		entityProvider:          entityProvider,
		isOrdered:               isOrdered,
		setOrdered:              setOrdered,
		isGenesis:               isGenesis,
		unorderedParentsCounter: memstorage.NewEpochStorage[IDType, uint8](),
		unorderedChildren:       memstorage.NewEpochStorage[IDType, []EntityType](),

		optsReferenceValidator: func(entity EntityType, parent EntityType) bool {
			return true
		},
	}, opts)
}

func (c *CausalOrder[IDType, EntityType]) Queue(entity EntityType) (ordered bool) {
	c.pruningMutex.RLock()
	defer c.pruningMutex.RUnlock()

	if entity.ID().Index() <= c.maxDroppedEpoch {
		c.Drop.Trigger(entity)
		return
	}

	if ordered = c.wasOrdered(entity); ordered {
		c.propagateCausalOrder(entity)
	}

	return ordered
}

func (c *CausalOrder[IDType, EntityType]) Prune(epochIndex epoch.Index) {
	for _, droppedEntity := range c.dropEntities(epochIndex) {
		c.Drop.Trigger(droppedEntity)
	}
}

func (c *CausalOrder[IDType, EntityType]) dropEntities(epochIndex epoch.Index) (droppedEntities map[IDType]EntityType) {
	c.pruningMutex.Lock()
	defer c.pruningMutex.Unlock()

	droppedEntities = make(map[IDType]EntityType)
	if epochIndex <= c.maxDroppedEpoch {
		return droppedEntities
	}

	for currentEpoch := c.maxDroppedEpoch + 1; currentEpoch <= epochIndex; currentEpoch++ {
		c.dropEntitiesFromEpoch(currentEpoch, func(id IDType) {
			if _, exists := droppedEntities[id]; !exists {
				droppedEntities[id] = c.entity(id)
			}
		})
	}
	c.maxDroppedEpoch = epochIndex

	return droppedEntities
}

func (c *CausalOrder[IDType, EntityType]) dropEntitiesFromEpoch(epochIndex epoch.Index, droppedEntityCallback func(id IDType)) {
	c.unorderedChildren.Get(epochIndex).ForEachKey(func(id IDType) bool {
		droppedEntityCallback(id)
		return true
	})
	c.unorderedChildren.Drop(epochIndex)

	c.unorderedParentsCounter.Get(epochIndex).ForEachKey(func(id IDType) bool {
		droppedEntityCallback(id)
		return true
	})
	c.unorderedParentsCounter.Drop(epochIndex)
}

func (c *CausalOrder[IDType, EntityType]) wasOrdered(entity EntityType) (wasOrdered bool) {
	c.lockEntity(entity)
	defer c.unlockEntity(entity)

	wasOrdered, isInvalid := c.checkEntity(entity)
	if isInvalid {
		c.Drop.Trigger(entity)
		return
	}

	if wasOrdered {
		c.Emit.Trigger(entity)
	}

	return
}

func (c *CausalOrder[IDType, EntityType]) checkEntity(entity EntityType) (wasOrdered bool, isInvalid bool) {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	countersStorage := c.unorderedParentsCounter.Get(entity.ID().Index(), true)
	unorderedParents, isInvalid := c.checkParents(entity)
	countersStorage.Set(entity.ID(), unorderedParents)
	if !isInvalid && unorderedParents == 0 {
		wasOrdered = c.setOrdered(entity, true)
	}

	return wasOrdered, isInvalid
}

func (c *CausalOrder[IDType, EntityType]) checkParents(entity EntityType) (unorderedParentsCount uint8, anyParentInvalid bool) {
	for _, parentID := range entity.ParentIDs() {
		parentEntity := c.entity(parentID)

		if !c.optsReferenceValidator(entity, parentEntity) || (!c.isGenesis(parentID) && parentID.Index() <= c.maxDroppedEpoch) {
			return unorderedParentsCount, true
		}

		if !c.isOrdered(parentEntity) {
			unorderedParentsCount++

			c.registerUnorderedChild(parentID, entity)
		}
	}

	return unorderedParentsCount, false
}

func (c *CausalOrder[IDType, EntityType]) registerUnorderedChild(entityID IDType, child EntityType) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	unorderedChildrenStorage := c.unorderedChildren.Get(entityID.Index(), true)

	entityChildren, _ := unorderedChildrenStorage.Get(entityID)
	unorderedChildrenStorage.Set(entityID, append(entityChildren, child))

}

func (c *CausalOrder[IDType, EntityType]) popUnorderedChildren(entityID IDType) (pendingChildren []EntityType) {
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

func (c *CausalOrder[IDType, EntityType]) propagateCausalOrder(metadata EntityType) {
	for childWalker := walker.New[IDType](true).Push(metadata.ID()); childWalker.HasNext(); {
		for _, child := range c.popUnorderedChildren(childWalker.Next()) {
			if c.triggerChildIfReady(child) {
				childWalker.Push(child.ID())
			}
		}
	}
}

func (c *CausalOrder[IDType, EntityType]) triggerChildIfReady(metadata EntityType) (eventTriggered bool) {
	metadata.Lock()
	defer metadata.Unlock()

	if c.decreaseUnorderedParentsCounter(metadata) != 0 || !c.setOrdered(metadata, true) {
		return false
	}

	c.Emit.Trigger(metadata)

	return true
}

func (c *CausalOrder[IDType, EntityType]) decreaseUnorderedParentsCounter(metadata EntityType) (unorderedParentsCounter uint8) {
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

func (c *CausalOrder[IDType, EntityType]) lockEntity(entity EntityType) {
	for _, parentID := range entity.ParentIDs() {
		c.entity(parentID).RLock()
	}

	entity.Lock()
}

func (c *CausalOrder[IDType, EntityType]) unlockEntity(metadata EntityType) {
	for _, parentID := range metadata.ParentIDs() {
		c.entity(parentID).RUnlock()
	}

	metadata.Unlock()
}

func (c *CausalOrder[IDType, EntityType]) entity(blockID IDType) (entity EntityType) {
	entity, exists := c.entityProvider(blockID)
	if !exists {
		panic(errors.Errorf("block %s does not exist", blockID))
	}

	return entity
}
