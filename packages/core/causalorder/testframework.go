package causalorder

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test                 *testing.T
	idCounter            int
	idCounterMutex       sync.Mutex
	entitiesByAlias      map[string]*MockOrderedEntity
	orderedEntities      map[string]bool
	orderedEntitiesMutex sync.RWMutex
	evictedEntities      map[string]bool
	evictedEntitiesMutex sync.RWMutex

	*CausalOrder[MockEntityID, *MockOrderedEntity]
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:            test,
		entitiesByAlias: make(map[string]*MockOrderedEntity),
		orderedEntities: make(map[string]bool),
		evictedEntities: make(map[string]bool),
	}, opts, func(t *TestFramework) {
		t.CausalOrder = New[MockEntityID, *MockOrderedEntity](
			eviction.NewManager[MockEntityID](func(id MockEntityID) (isRootBlock bool) {
				return id.id == 0
			}),
			func(id MockEntityID) (entity *MockOrderedEntity, exists bool) {
				return t.Get(id.alias)
			}, func(entity *MockOrderedEntity) (isOrdered bool) {
				return entity.ordered
			}, func(entity *MockOrderedEntity) (err error) {
				entity.ordered = true

				if debug.GetEnabled() {
					t.test.Logf("%s ordered", entity.id.alias)
				}

				t.orderedEntitiesMutex.Lock()
				t.orderedEntities[entity.id.alias] = true
				t.orderedEntitiesMutex.Unlock()

				return nil
			}, func(entity *MockOrderedEntity, reason error) {
				entity.invalid = true

				if debug.GetEnabled() {
					t.test.Logf("%s evicted", entity.id.alias)
				}

				t.evictedEntitiesMutex.Lock()
				t.evictedEntities[entity.id.alias] = true
				t.evictedEntitiesMutex.Unlock()
			},
			WithReferenceValidator[MockEntityID, *MockOrderedEntity](func(entity, parent *MockOrderedEntity) (err error) {
				if entity.invalid {
					return errors.Errorf("entity %s is invalid", entity.id.alias)
				}

				if parent.invalid {
					return errors.Errorf("parent %s of entity %s is invalid", parent.id.alias, entity.id.alias)
				}

				return checkReference[MockEntityID, *MockOrderedEntity](entity, parent)
			}),
		)

		t.CreateEntity("Genesis", WithOrdered(true), WithEpoch(0))
	})
}

// CreateEntity creates a Entity with the given alias and options.
func (t *TestFramework) CreateEntity(alias string, opts ...options.Option[MockOrderedEntity]) (entity *MockOrderedEntity) {
	entity = NewMockOrderedEntity(NewID(t.nextID()), opts...)
	entity.id.alias = alias

	t.entitiesByAlias[alias] = entity

	return
}

func (t *TestFramework) nextID() (nextID int) {
	t.idCounterMutex.Lock()
	defer t.idCounterMutex.Unlock()

	nextID = t.idCounter
	t.idCounter++

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

func (t *TestFramework) EvictEpoch(index epoch.Index) {
	t.evictionManager.EvictEpoch(index)
	t.CausalOrder.EvictEpoch(index)
}

func (t *TestFramework) AssertOrdered(aliases ...string) {
	t.orderedEntitiesMutex.RLock()
	defer t.orderedEntitiesMutex.RUnlock()

	require.Equal(t.test, len(aliases), len(t.orderedEntities))

	for _, alias := range aliases {
		require.True(t.test, t.orderedEntities[alias])
	}
}

func (t *TestFramework) AssertEvicted(aliases ...string) {
	t.evictedEntitiesMutex.RLock()
	defer t.evictedEntitiesMutex.RUnlock()

	require.Equal(t.test, len(aliases), len(t.evictedEntities))

	for _, alias := range aliases {
		require.True(t.test, t.evictedEntities[alias])
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Mocks ////////////////////////////////////////////////////////////////////////////////////////////////////////

type MockEntityID struct {
	id    int
	index epoch.Index
	alias string
}

func NewID(id int) MockEntityID {
	return MockEntityID{
		id:    id,
		index: 1,
	}
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
	return options.Apply(&MockOrderedEntity{
		id: id,
	}, opts)
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
