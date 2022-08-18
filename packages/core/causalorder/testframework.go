package causalorder

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type TestFramework struct {
	entitiesByAlias map[string]*MockOrderedEntity
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(opts ...options.Option[TestFramework]) (newFramework *TestFramework) {
	return options.Apply(&TestFramework{
		entitiesByAlias: make(map[string]*MockOrderedEntity),
	}, opts)
}

// CreateEntity creates a Entity with the given alias and options.
func (t *TestFramework) CreateEntity(alias string, id int, opts ...options.Option[MockOrderedEntity]) (entity *MockOrderedEntity) {
	entity = NewMockOrderedEntity(NewID(id), opts...)
	entity.id.alias = alias

	t.entitiesByAlias[alias] = entity

	return
}

// Entity retrieves the Entity that is associated with the given alias.
func (t *TestFramework) Entity(alias string) (entity *MockOrderedEntity) {
	entity, ok := t.entitiesByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Entity alias %s not registered", alias))
	}
	return
}

// Get retrieves the Entity that is associated with the given alias.
func (t *TestFramework) Get(alias string) (entity *MockOrderedEntity, ok bool) {
	entity, ok = t.entitiesByAlias[alias]
	return
}

// EntityIDs retrieves the EntityIDs that are associated with the given aliases.
func (t *TestFramework) EntityIDs(aliases ...string) (entityIDs []MockEntityID) {
	entityIDs = make([]MockEntityID, len(aliases))
	for i, alias := range aliases {
		entity := t.Entity(alias)
		entityIDs[i] = entity.ID()
	}

	return
}

// region Mocks ////////////////////////////////////////////////////////////////////////////////////////////////////////

type MockEntityID struct {
	id    int
	index epoch.Index
	alias string
}

func NewID(id int) MockEntityID {
	entityID := MockEntityID{id: id}
	return entityID
}

func (m MockEntityID) Index() epoch.Index {
	return m.index
}

func (m MockEntityID) String() string {
	return fmt.Sprintf("MockIndexedID(%s)", m.alias)
}

type MockOrderedEntity struct {
	id      MockEntityID
	parents []MockEntityID
	ordered bool
	invalid bool
}

func NewMockOrderedEntity(id MockEntityID, opts ...options.Option[MockOrderedEntity]) *MockOrderedEntity {
	entity := &MockOrderedEntity{id: id}
	options.Apply(entity, opts)
	return entity
}

func (m MockOrderedEntity) ID() MockEntityID {
	return m.id
}

func (m MockOrderedEntity) Parents() []MockEntityID {
	return m.parents
}

func WithParents(parents []MockEntityID) options.Option[MockOrderedEntity] {
	return func(entity *MockOrderedEntity) {
		entity.parents = parents
	}
}

func WithOrdered(ordered bool) options.Option[MockOrderedEntity] {
	return func(entity *MockOrderedEntity) {
		entity.ordered = ordered
	}
}

func WithInvalid(invalid bool) options.Option[MockOrderedEntity] {
	return func(entity *MockOrderedEntity) {
		entity.invalid = invalid
	}
}

func WithEpoch(index epoch.Index) options.Option[MockOrderedEntity] {
	return func(entity *MockOrderedEntity) {
		entity.id.index = index
	}
}

// endregion //////////////////////////////////////////////////////////////////////////////////////////////////////////
