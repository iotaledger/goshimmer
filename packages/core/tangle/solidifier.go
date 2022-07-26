package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/walker"
)

type CausallyOrderedEntity[IDType comparable] interface {
	ID() IDType
	ParentIDs() []IDType

	RLock()
	RUnlock()
	Lock()
	Unlock()
}

type EntityProvider[IDType comparable, EntityType CausallyOrderedEntity[IDType]] func(IDType) (entity EntityType, exists bool)

type CausalOrderer[IDType comparable, EntityType CausallyOrderedEntity[IDType]] struct {
	ElementOrdered *event.Event[EntityType]
	ElementInvalid *event.Event[EntityType]

	entityProvider               EntityProvider[IDType, EntityType]
	isOrdered                    func(EntityType) bool
	setOrdered                   func(EntityType, bool) bool
	unorderedParentsCounter      map[IDType]uint8
	unorderedParentsCounterMutex sync.Mutex
	unorderedChildren            map[IDType][]EntityType
	unorderedChildrenMutex       sync.Mutex
}

func NewCausalOrderer[IDType comparable, EntityType CausallyOrderedEntity[IDType]](
	entityProvider EntityProvider[IDType, EntityType],
	orderedFlagGetter func(EntityType) bool,
	orderedFlagSetter func(EntityType, bool) bool,
) *CausalOrderer[IDType, EntityType] {
	return &CausalOrderer[IDType, EntityType]{
		entityProvider:          entityProvider,
		isOrdered:               orderedFlagGetter,
		setOrdered:              orderedFlagSetter,
		unorderedParentsCounter: make(map[IDType]uint8),
		unorderedChildren:       make(map[IDType][]EntityType),
	}
}

func (c *CausalOrderer[IDType, EntityType]) Queue(entity EntityType) (ordered bool) {
	if ordered = c.wasOrdered(entity); ordered {
		c.propagateCausalOrder(entity)
	}

	return ordered
}

func (c *CausalOrderer[IDType, EntityType]) wasOrdered(entity EntityType) (wasOrdered bool) {
	c.lockEntity(entity)
	defer c.unlockEntity(entity)

	isInvalid := false

	c.unorderedParentsCounterMutex.Lock()
	c.unorderedParentsCounter[entity.ID()], isInvalid = c.checkParents(entity)
	if !isInvalid && c.unorderedParentsCounter[entity.ID()] == 0 {
		c.setOrdered(entity, true)
	}
	c.unorderedParentsCounterMutex.Unlock()

	if isInvalid {
		c.ElementInvalid.Trigger(entity)
	}

	if wasOrdered = c.isOrdered(entity); wasOrdered {
		c.ElementOrdered.Trigger(entity)
	}

	return
}

func (c *CausalOrderer[IDType, EntityType]) checkParents(metadata EntityType) (unorderedParentsCount uint8, anyParentInvalid bool) {
	for _, parentID := range metadata.ParentIDs() {
		parentMetadata := c.entity(parentID)
		if parentMetadata.invalid {
			return unorderedParentsCount, true
		}

		if parentMetadata.missing || !c.isOrdered(parentMetadata) {
			unorderedParentsCount++

			c.registerUnorderedChild(parentID, metadata)
		}
	}

	return unorderedParentsCount, false
}

func (c *CausalOrderer[IDType, EntityType]) registerUnorderedChild(entityID IDType, child EntityType) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	c.unorderedChildren[entityID] = append(c.unorderedChildren[entityID], child)

}

func (c *CausalOrderer[IDType, EntityType]) popUnorderedChildren(entityID IDType) (pendingChildren []EntityType) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	pendingChildren = c.unorderedChildren[entityID]
	delete(c.unorderedChildren, entityID)

	return pendingChildren
}

func (c *CausalOrderer[IDType, EntityType]) propagateCausalOrder(metadata EntityType) {
	for childWalker := walker.New[IDType](true).Push(metadata.ID()); childWalker.HasNext(); {
		for _, child := range c.popUnorderedChildren(childWalker.Next()) {
			if c.triggerChildIfReady(child) {
				childWalker.Push(child.ID())
			}
		}
	}
}

func (c *CausalOrderer[IDType, EntityType]) triggerChildIfReady(metadata EntityType) (eventTriggered bool) {
	metadata.Lock()
	defer metadata.Unlock()

	if !c.isReady(metadata) {
		return false
	}

	c.setOrdered(metadata, true)

	c.ElementOrdered.Trigger(metadata)

	return true
}

func (c *CausalOrderer[IDType, EntityType]) isReady(metadata EntityType) bool {
	c.unorderedParentsCounterMutex.Lock()
	defer c.unorderedParentsCounterMutex.Unlock()

	c.unorderedParentsCounter[metadata.ID()]--

	return c.unorderedParentsCounter[metadata.ID()] == 0
}

func (c *CausalOrderer[IDType, EntityType]) lockEntity(entity EntityType) {
	for _, parentID := range entity.ParentIDs() {
		c.entity(parentID).RLock()
	}

	entity.Lock()
}

func (c *CausalOrderer[IDType, EntityType]) unlockEntity(metadata EntityType) {
	for _, parentID := range metadata.ParentIDs() {
		c.entity(parentID).RUnlock()
	}

	metadata.Unlock()
}

func (c *CausalOrderer[IDType, EntityType]) entity(blockID IDType) (entity EntityType) {
	entity, exists := c.entityProvider(blockID)
	if !exists {
		panic(errors.Errorf("block %s does not exist", blockID))
	}

	return entity
}
