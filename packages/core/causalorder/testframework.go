package causalorder

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework is the test framework for the CausalOrder.
type TestFramework struct {
	test                 *testing.T
	idCounter            int
	idCounterMutex       sync.Mutex
	entitiesByAlias      map[string]*MockedOrderedEntity
	orderedEntities      map[string]*MockedOrderedEntity
	orderedEntitiesMutex sync.RWMutex
	evictedEntities      map[string]*MockedOrderedEntity
	evictedEntitiesMutex sync.RWMutex

	*CausalOrder[MockedEntityID, *MockedOrderedEntity]
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:            test,
		entitiesByAlias: make(map[string]*MockedOrderedEntity),
		orderedEntities: make(map[string]*MockedOrderedEntity),
		evictedEntities: make(map[string]*MockedOrderedEntity),
	}, opts, func(t *TestFramework) {
		t.CausalOrder = New[MockedEntityID, *MockedOrderedEntity](
			eviction.NewState[MockedEntityID](),
			func(id MockedEntityID) (entity *MockedOrderedEntity, exists bool) {
				return t.Get(id.alias)
			}, (*MockedOrderedEntity).IsOrdered,
			func(entity *MockedOrderedEntity) (err error) {
				if !entity.SetOrdered(true) {
					return nil
				}

				if debug.GetEnabled() {
					t.test.Logf("%s ordered", entity.id.alias)
				}

				t.orderedEntitiesMutex.Lock()
				t.orderedEntities[entity.id.alias] = entity
				t.orderedEntitiesMutex.Unlock()

				return nil
			}, func(entity *MockedOrderedEntity, reason error) {
				if !entity.SetInvalid(true) {
					return
				}

				if debug.GetEnabled() {
					t.test.Logf("%s evicted", entity.id.alias)
				}

				t.evictedEntitiesMutex.Lock()
				t.evictedEntities[entity.id.alias] = entity
				t.evictedEntitiesMutex.Unlock()
			},
			WithReferenceValidator[MockedEntityID, *MockedOrderedEntity](func(entity, parent *MockedOrderedEntity) (err error) {
				if entity.IsInvalid() {
					return errors.Errorf("entity %s is invalid", entity.id.alias)
				}

				if parent.IsInvalid() {
					return errors.Errorf("parent %s of entity %s is invalid", parent.id.alias, entity.id.alias)
				}

				return checkReference[MockedEntityID, *MockedOrderedEntity](entity, parent)
			}),
		)

		t.CreateEntity("Genesis", WithOrdered(true), WithEpoch(0))
	})
}

// CreateEntity creates a Entity with the given alias and options.
func (t *TestFramework) CreateEntity(alias string, opts ...options.Option[MockedOrderedEntity]) (entity *MockedOrderedEntity) {
	entity = NewMockOrderedEntity(NewMockedEntityID(t.nextID()), opts...)
	entity.id.alias = alias

	t.entitiesByAlias[alias] = entity

	return
}

// RemoveEntity removes the Entity that is associated with the given alias.
func (t *TestFramework) RemoveEntity(alias string) {
	delete(t.entitiesByAlias, alias)
}

// nextID returns the next ID and increases the idCounter.
func (t *TestFramework) nextID() (nextID int) {
	t.idCounterMutex.Lock()
	defer t.idCounterMutex.Unlock()

	nextID = t.idCounter
	t.idCounter++

	return
}

// Entity retrieves the Entity that is associated with the given alias.
func (t *TestFramework) Entity(alias string) (entity *MockedOrderedEntity) {
	entity, ok := t.entitiesByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Entity alias %s not registered", alias))
	}
	return
}

// Get retrieves the Entity that is associated with the given alias.
func (t *TestFramework) Get(alias string) (entity *MockedOrderedEntity, ok bool) {
	entity, ok = t.entitiesByAlias[alias]
	return
}

// EntityIDs retrieves the EntityIDs that are associated with the given aliases.
func (t *TestFramework) EntityIDs(aliases ...string) (entityIDs []MockedEntityID) {
	entityIDs = make([]MockedEntityID, len(aliases))
	for i, alias := range aliases {
		entity := t.Entity(alias)
		entityIDs[i] = entity.ID()
	}

	return
}

// EvictEpoch evicts all Entities that are older than the given epoch.
func (t *TestFramework) EvictEpoch(index epoch.Index) {
	t.evictionManager.EvictUntil(index, set.NewAdvancedSet[MockedEntityID]())
	t.CausalOrder.EvictEpoch(index)
}

// AssertOrdered asserts that the given Entities are ordered.
func (t *TestFramework) AssertOrdered(aliases ...string) {
	t.orderedEntitiesMutex.RLock()
	defer t.orderedEntitiesMutex.RUnlock()

	require.Equal(t.test, len(aliases), len(t.orderedEntities))

	for _, alias := range aliases {
		entity, exists := t.orderedEntities[alias]
		require.True(t.test, exists)
		require.True(t.test, entity.IsOrdered())
	}
}

// AssertEvicted asserts that the given Entities are evicted.
func (t *TestFramework) AssertEvicted(aliases ...string) {
	t.evictedEntitiesMutex.RLock()
	defer t.evictedEntitiesMutex.RUnlock()

	require.Equal(t.test, len(aliases), len(t.evictedEntities))

	for _, alias := range aliases {
		entity, exists := t.evictedEntities[alias]
		require.True(t.test, exists)
		require.True(t.test, entity.IsInvalid())
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedEntityID ///////////////////////////////////////////////////////////////////////////////////////////////

// MockedEntityID is a mocked EntityID.
type MockedEntityID struct {
	id    int
	index epoch.Index
	alias string
}

// NewMockedEntityID creates a new MockedEntityID.
func NewMockedEntityID(id int) MockedEntityID {
	return MockedEntityID{
		id:    id,
		index: 1,
	}
}

// Index returns the epoch.Index of the Entity.
func (m MockedEntityID) Index() epoch.Index {
	return m.index
}

// String returns a human-readable representation of the EntityID.
func (m MockedEntityID) String() string {
	return fmt.Sprintf("MockIndexedID(%s)", m.alias)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedOrderedEntity //////////////////////////////////////////////////////////////////////////////////////////

// MockedOrderedEntity is a mocked OrderedEntity.
type MockedOrderedEntity struct {
	// id contains the identifier of the Entity.
	id MockedEntityID

	// parents contains the parents of the Entity.
	parents []MockedEntityID

	// ordered contains a flag that indicates whether the Entity was ordered.
	ordered bool

	// invalid contains a flag that indicates whether the Entity was marked as invalid.
	invalid bool

	// RWMutex is used to synchronize access to the Entity.
	sync.RWMutex
}

// NewMockOrderedEntity returns a new MockedOrderedEntity.
func NewMockOrderedEntity(id MockedEntityID, opts ...options.Option[MockedOrderedEntity]) *MockedOrderedEntity {
	return options.Apply(&MockedOrderedEntity{
		id: id,
	}, opts)
}

// ID returns the identifier of the Entity.
func (m *MockedOrderedEntity) ID() MockedEntityID {
	return m.id
}

// Parents returns the parents of the Entity.
func (m *MockedOrderedEntity) Parents() []MockedEntityID {
	return m.parents
}

// IsOrdered returns a flag that indicates whether the Entity was ordered.
func (m *MockedOrderedEntity) IsOrdered() (isOrdered bool) {
	m.RLock()
	defer m.RUnlock()

	return m.ordered
}

// SetOrdered sets the ordered flag of the Entity.
func (m *MockedOrderedEntity) SetOrdered(ordered bool) (updated bool) {
	m.Lock()
	defer m.Unlock()

	if m.ordered == ordered {
		return false
	}
	m.ordered = ordered

	return true
}

// IsInvalid returns a flag that indicates whether the Entity was marked as invalid.
func (m *MockedOrderedEntity) IsInvalid() (isInvalid bool) {
	m.RLock()
	defer m.RUnlock()

	return m.invalid
}

// SetInvalid sets the invalid flag of the Entity.
func (m *MockedOrderedEntity) SetInvalid(invalid bool) (updated bool) {
	m.Lock()
	defer m.Unlock()

	if m.invalid == invalid {
		return false
	}
	m.invalid = invalid

	return true
}

// endregion //////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithParents is an option that sets the parents of the Entity.
func WithParents(parents []MockedEntityID) options.Option[MockedOrderedEntity] {
	return func(entity *MockedOrderedEntity) {
		entity.parents = parents
	}
}

// WithOrdered is an option that sets the ordered flag of the Entity.
func WithOrdered(ordered bool) options.Option[MockedOrderedEntity] {
	return func(entity *MockedOrderedEntity) {
		entity.ordered = ordered
	}
}

// WithInvalid is an option that sets the invalid flag of the Entity.
func WithInvalid(invalid bool) options.Option[MockedOrderedEntity] {
	return func(entity *MockedOrderedEntity) {
		entity.invalid = invalid
	}
}

// WithEpoch is an option that sets the epoch of the Entity.
func WithEpoch(index epoch.Index) options.Option[MockedOrderedEntity] {
	return func(entity *MockedOrderedEntity) {
		entity.id.index = index
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
