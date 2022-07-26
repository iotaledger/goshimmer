package causalorder

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/generics/walker"
)

type CausalOrder[IDType comparable, EntityType Entity[IDType]] struct {
	Emit *event.Event[EntityType]
	Drop *event.Event[EntityType]

	entityProvider EntityProvider[IDType, EntityType]
	isOrdered      func(EntityType) bool
	setOrdered     func(EntityType, bool) bool

	unorderedParentsCounter      map[IDType]uint8
	unorderedParentsCounterMutex sync.Mutex
	unorderedChildren            map[IDType][]EntityType
	unorderedChildrenMutex       sync.Mutex

	optsReferenceValidator func(entity EntityType, parent EntityType) bool
}

func New[IDType comparable, EntityType Entity[IDType]](
	entityProvider EntityProvider[IDType, EntityType],
	orderedFlagGetter func(EntityType) bool,
	orderedFlagSetter func(EntityType, bool) bool,
	opts ...options.Option[CausalOrder[IDType, EntityType]],
) *CausalOrder[IDType, EntityType] {
	return options.Apply(&CausalOrder[IDType, EntityType]{
		Emit: event.New[EntityType](),
		Drop: event.New[EntityType](),

		entityProvider:          entityProvider,
		isOrdered:               orderedFlagGetter,
		setOrdered:              orderedFlagSetter,
		unorderedParentsCounter: make(map[IDType]uint8),
		unorderedChildren:       make(map[IDType][]EntityType),

		optsReferenceValidator: func(entity EntityType, parent EntityType) bool {
			return true
		},
	}, opts)
}

func (c *CausalOrder[IDType, EntityType]) Queue(entity EntityType) (ordered bool) {
	if ordered = c.wasOrdered(entity); ordered {
		c.propagateCausalOrder(entity)
	}

	return ordered
}

func (c *CausalOrder[IDType, EntityType]) wasOrdered(entity EntityType) (wasOrdered bool) {
	c.lockEntity(entity)
	defer c.unlockEntity(entity)

	isInvalid := false

	c.unorderedParentsCounterMutex.Lock()
	c.unorderedParentsCounter[entity.ID()], isInvalid = c.checkParents(entity)
	if !isInvalid && c.unorderedParentsCounter[entity.ID()] == 0 {
		wasOrdered = c.setOrdered(entity, true)
	}
	c.unorderedParentsCounterMutex.Unlock()

	if isInvalid {
		c.Drop.Trigger(entity)
		return
	}

	if wasOrdered {
		c.Emit.Trigger(entity)
	}

	return
}

func (c *CausalOrder[IDType, EntityType]) checkParents(entity EntityType) (unorderedParentsCount uint8, anyParentInvalid bool) {
	for _, parentID := range entity.ParentIDs() {
		parentEntity := c.entity(parentID)

		if !c.optsReferenceValidator(entity, parentEntity) {
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

	c.unorderedChildren[entityID] = append(c.unorderedChildren[entityID], child)

}

func (c *CausalOrder[IDType, EntityType]) popUnorderedChildren(entityID IDType) (pendingChildren []EntityType) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	pendingChildren = c.unorderedChildren[entityID]
	delete(c.unorderedChildren, entityID)

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

	c.unorderedParentsCounter[metadata.ID()]--

	return c.unorderedParentsCounter[metadata.ID()]
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
