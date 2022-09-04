package eviction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type Manager[ID epoch.IndexedID] struct {
	Events *Events

	maxEvictedEpoch epoch.Index
	isRootBlock     func(ID) bool

	sync.RWMutex
}

func NewManager[ID epoch.IndexedID](isRootBlock func(ID) (isRootBlock bool)) (newManager *Manager[ID]) {
	return &Manager[ID]{
		Events:      NewEvents(),
		isRootBlock: isRootBlock,
	}
}

// Lockable returns a lockable version of the Manager that contains an additional mutex used to synchronize the eviction
// process inside the components.
func (m *Manager[ID]) Lockable() (newLockableManager *LockableManager[ID]) {
	return &LockableManager[ID]{
		Manager: m,
	}
}

func (m *Manager[ID]) EvictEpoch(epochIndex epoch.Index) {
	for currentIndex := m.setMaxEvictedEpoch(epochIndex) + 1; currentIndex <= epochIndex; currentIndex++ {
		m.Events.EpochEvicted.Trigger(currentIndex)
	}
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (m *Manager[ID]) IsTooOld(id ID) (isTooOld bool) {
	m.RLock()
	defer m.RUnlock()

	return !m.isRootBlock(id) && id.Index() <= m.maxEvictedEpoch
}

func (m *Manager[ID]) IsRootBlock(id ID) (isRootBlock bool) {
	return m.isRootBlock(id)
}

func (m *Manager[ID]) MaxEvictedEpoch() epoch.Index {
	m.RLock()
	defer m.RUnlock()

	return m.maxEvictedEpoch
}

func (m *Manager[ID]) setMaxEvictedEpoch(index epoch.Index) (old epoch.Index) {
	m.Lock()
	defer m.Unlock()

	if old = m.maxEvictedEpoch; old >= index {
		return
	}

	m.maxEvictedEpoch = index

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
