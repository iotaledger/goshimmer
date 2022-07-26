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
	orderedFlagGetter            func(EntityType) bool
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
		orderedFlagGetter:       orderedFlagGetter,
		setOrdered:              orderedFlagSetter,
		unorderedParentsCounter: make(map[IDType]uint8),
		unorderedChildren:       make(map[IDType][]EntityType),
	}
}

func (t *CausalOrderer[IDType, EntityType]) Solidify(entity EntityType) {
	becameSolid, becameInvalid := t.becameSolidOrInvalid(entity)
	if becameInvalid {
		t.propagateInvalidityToChildren(entity)
	}

	if becameSolid {
		t.propagateSolidity(entity)
	}
}

func (t *CausalOrderer[IDType, EntityType]) becameSolidOrInvalid(entity EntityType) (becameSolid, becameInvalid bool) {
	t.lockEntity(entity)
	defer t.unlockEntity(entity)

	t.unorderedParentsCounterMutex.Lock()
	t.unorderedParentsCounter[entity.ID()], entity.invalid = t.checkParents(entity)
	if !entity.invalid && t.unorderedParentsCounter[entity.ID()] == 0 {
		t.setOrdered(entity, true)
	}
	t.unorderedParentsCounterMutex.Unlock()

	if entity.invalid {
		t.ElementInvalid.Trigger(entity)
	}

	if entity.solid {
		t.Events.BlockSolid.Trigger(entity)
	}

	// only one of the returned values can be true
	return entity.solid, entity.invalid
}

func (t *CausalOrderer[IDType, EntityType]) checkParents(metadata EntityType) (unsolidParents uint8, anyParentInvalid bool) {
	for parentID := range metadata.ParentIDs() {
		parentMetadata := t.entity(parentID)
		if parentMetadata.invalid {
			return unsolidParents, true
		}
		if parentMetadata.missing || !parentMetadata.solid {
			t.childDependenciesMutex.Lock()
			t.childDependencies[parentID] = append(t.childDependencies[parentID], metadata)
			t.childDependenciesMutex.Unlock()

			unsolidParents++
		}
	}

	return unsolidParents, false
}

func (c *CausalOrderer[IDType, EntityType]) pendingChildren(entityID IDType) (pendingChildren []EntityType) {
	c.unorderedChildrenMutex.Lock()
	defer c.unorderedChildrenMutex.Unlock()

	pendingChildren = c.unorderedChildren[entityID]
	delete(c.unorderedChildren, entityID)

	return pendingChildren
}

func (t *CausalOrderer[IDType, EntityType]) propagateSolidity(metadata EntityType) {
	for childWalker := walker.New[IDType](true).Push(metadata.ID()); childWalker.HasNext(); {
		for _, child := range t.pendingChildren(childWalker.Next()) {
			if t.decreaseUnsolidParentsCounter(child) {
				childWalker.Push(child.ID())
			}
		}
	}
}

func (t *CausalOrderer[IDType, EntityType]) propagateInvalidityToChildren(entity EntityType) {
	for propagationWalker := walker.New[EntityType](true).Push(entity); propagationWalker.HasNext(); {
		for _, childMetadata := range propagationWalker.Next().Children() {
			if childMetadata.setInvalid() {
				t.Events.BlockInvalid.Trigger(childMetadata)

				propagationWalker.Push(childMetadata)
			}
		}
	}
}

func (t *CausalOrderer[IDType, EntityType]) decreaseUnsolidParentsCounter(metadata EntityType) bool {
	metadata.Lock()
	defer metadata.Unlock()

	t.unsolidDependenciesCounterMutex.Lock()
	t.unsolidDependenciesCounter[metadata.id]--
	metadata.solid = t.unsolidDependenciesCounter[metadata.id] == 0
	t.unsolidDependenciesCounterMutex.Unlock()

	if !metadata.solid {
		return false
	}

	t.Events.BlockSolid.Trigger(metadata)

	return true
}

func (s *CausalOrderer[IDType, EntityType]) lockEntity(entity EntityType) {
	for parentID := range entity.ParentIDs() {
		s.entityProvider(parentID).RLock()
	}

	entity.Lock()
}

func (s *CausalOrderer[IDType, EntityType]) unlockEntity(metadata EntityType) {
	for parentID := range metadata.ParentIDs() {
		s.entityProvider(parentID).RUnlock()
	}

	metadata.Unlock()
}

func (s *CausalOrderer[IDType, EntityType]) entity(blockID IDType) (entity EntityType) {
	if entity, exists := s.entityProvider(blockID); exists {
		return entity
	}

	panic(errors.Errorf("block %s does not exist", blockID))
}
