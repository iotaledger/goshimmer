package eviction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type Manager struct {
	Events *Events

	maxDroppedEpoch epoch.Index
	isRootBlock     func(models.BlockID) bool

	sync.RWMutex
}

func NewManager(isRootBlock func(models.BlockID) (isRootBlock bool)) (newManager *Manager) {
	return &Manager{
		Events:      newEvents(),
		isRootBlock: isRootBlock,
	}
}

// Lockable returns a lockable version of the Manager that contains an additional mutex used to synchronize the eviction
// process inside the components.
func (m *Manager) Lockable() (newLockableManager *LockableManager) {
	return &LockableManager{
		Manager: m,
	}
}

func (m *Manager) Evict(epochIndex epoch.Index) {
	for currentIndex := m.setMaxDroppedEpoch(epochIndex) + 1; currentIndex <= epochIndex; currentIndex++ {
		m.Events.EpochEvicted.Trigger(currentIndex)
	}
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (m *Manager) IsTooOld(id models.BlockID) (isTooOld bool) {
	m.RLock()
	defer m.RUnlock()

	return !m.isRootBlock(id) && id.EpochIndex <= m.maxDroppedEpoch
}

func (m *Manager) IsRootBlock(id models.BlockID) (isRootBlock bool) {
	return m.isRootBlock(id)
}

func (m *Manager) MaxDroppedEpoch() epoch.Index {
	m.RLock()
	defer m.RUnlock()

	return m.maxDroppedEpoch
}

func (m *Manager) setMaxDroppedEpoch(index epoch.Index) (old epoch.Index) {
	m.Lock()
	defer m.Unlock()

	if old = m.maxDroppedEpoch; old >= index {
		return
	}

	m.maxDroppedEpoch = index

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LockableManager //////////////////////////////////////////////////////////////////////////////////////////////

// LockableManager is a wrapper around the Manager that contains an additional Mutex used to synchronize the eviction
// process in the individual components.
type LockableManager struct {
	// Manager is the underlying Manager.
	*Manager

	// RWMutex is the mutex that is used to synchronize the eviction process.
	sync.RWMutex
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
