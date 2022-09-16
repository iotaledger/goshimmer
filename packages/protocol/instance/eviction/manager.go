package eviction

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type Manager[ID epoch.IndexedID] struct {
	Events *Events

	maxEvictedEpoch   epoch.Index
	currentRootBlocks *set.AdvancedSet[ID]

	rootBlockProvider func(index epoch.Index) *set.AdvancedSet[ID]

	sync.RWMutex
}

func NewManager[ID epoch.IndexedID](snapshotEpochIndex epoch.Index, rootBlockProvider func(index epoch.Index) *set.AdvancedSet[ID]) (newManager *Manager[ID]) {
	return &Manager[ID]{
		Events:            NewEvents(),
		currentRootBlocks: rootBlockProvider(snapshotEpochIndex),
		maxEvictedEpoch:   snapshotEpochIndex,
		rootBlockProvider: rootBlockProvider,
	}
}

// Lockable returns a lockable version of the Manager that contains an additional mutex used to synchronize the eviction
// process inside the components.
func (m *Manager[ID]) Lockable() (newLockableManager *LockableManager[ID]) {
	return &LockableManager[ID]{
		Manager: m,
	}
}

func (m *Manager[ID]) EvictUntilEpoch(epochIndex epoch.Index) {
	previousEvicted := m.setEvictedEpochAndUpdateRootBlocks(epochIndex)
	for currentIndex := previousEvicted + 1; currentIndex <= epochIndex; currentIndex++ {
		m.Events.EpochEvicted.Trigger(currentIndex)
	}
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (m *Manager[ID]) IsTooOld(id ID) (isTooOld bool) {
	m.RLock()
	defer m.RUnlock()

	return id.Index() <= m.maxEvictedEpoch && !m.isRootBlock(id)
}

func (m *Manager[ID]) IsRootBlock(id ID) (isRootBlock bool) {
	m.RLock()
	defer m.RUnlock()

	return m.isRootBlock(id)
}

func (m *Manager[ID]) RootBlocks() (rootBlocks *set.AdvancedSet[ID]) {
	m.RLock()
	defer m.RUnlock()

	return m.currentRootBlocks.Clone()
}

func (m *Manager[ID]) MaxEvictedEpoch() epoch.Index {
	m.RLock()
	defer m.RUnlock()

	return m.maxEvictedEpoch
}

func (m *Manager[ID]) isRootBlock(id ID) (isRootBlock bool) {
	return m.currentRootBlocks.Has(id)
}

// setEvictedEpochAndUpdateRootBlocks atomically increases maxEvictedEpoch and updates the root blocks for the epoch.
func (m *Manager[ID]) setEvictedEpochAndUpdateRootBlocks(index epoch.Index) (old epoch.Index) {
	m.Lock()
	defer m.Unlock()

	if old = m.maxEvictedEpoch; old >= index {
		return
	}

	m.maxEvictedEpoch = index
	m.currentRootBlocks = m.rootBlockProvider(index)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LockableManager //////////////////////////////////////////////////////////////////////////////////////////////

// LockableManager is a wrapper around the Manager that contains an additional Mutex used to synchronize the eviction
// process in the individual components.
type LockableManager[ID epoch.IndexedID] struct {
	// Manager is the underlying Manager.
	*Manager[ID]

	// RWMutex is the mutex that is used to synchronize the eviction process.
	sync.RWMutex
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
