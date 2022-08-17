package eviction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

type Manager struct {
	maxDroppedEpoch epoch.Index
	isRootBlock     func(models.BlockID) bool

	pruningMutex sync.RWMutex
}

func NewManager(rootBlockProvider func(models.BlockID) bool) *Manager {
	return &Manager{
		isRootBlock: rootBlockProvider,
	}
}

func (m *Manager) NewState() *State {
	return NewState(m)
}

func (m *Manager) Evict(epochIndex epoch.Index) {
	for i := m.setMaxDroppedEpoch(epochIndex) + 1; i <= epochIndex; i++ {
		// TODO: trigger event
	}
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (m *Manager) IsTooOld(id models.BlockID) (isTooOld bool) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	return !m.isRootBlock(id) && id.EpochIndex <= m.maxDroppedEpoch
}

func (m *Manager) IsRootBlock(id models.BlockID) (rootBlock bool) {
	return m.isRootBlock(id)
}

func (m *Manager) MaxDroppedEpoch() epoch.Index {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	return m.maxDroppedEpoch
}

func (m *Manager) setMaxDroppedEpoch(index epoch.Index) (old epoch.Index) {
	m.pruningMutex.Lock()
	defer m.pruningMutex.Unlock()

	if old = m.maxDroppedEpoch; old >= index {
		return
	}

	m.maxDroppedEpoch = index

	return
}
