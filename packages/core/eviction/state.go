package eviction

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type State[ID epoch.IndexedID] struct {
	Events *Events

	maxEvictedEpoch epoch.Index
	rootBlocks      *set.AdvancedSet[ID]

	sync.RWMutex
}

func NewState[ID epoch.IndexedID]() (newManager *State[ID]) {
	var emptyID ID

	return &State[ID]{
		Events:          NewEvents(),
		rootBlocks:      set.NewAdvancedSet(emptyID),
		maxEvictedEpoch: 0,
	}
}

// Lockable returns a lockable version of the Manager that contains an additional mutex used to synchronize the eviction
// process inside the components.
func (m *State[ID]) Lockable() (newLockableManager *LockableManager[ID]) {
	return &LockableManager[ID]{
		State: m,
	}
}

func (m *State[ID]) EvictUntil(epochIndex epoch.Index, rootBlocks *set.AdvancedSet[ID]) {
	previousEvicted := m.setEvictedEpochAndUpdateRootBlocks(epochIndex, rootBlocks)

	for currentIndex := previousEvicted + 1; currentIndex <= epochIndex; currentIndex++ {
		m.Events.EpochEvicted.Trigger(currentIndex)
	}
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (m *State[ID]) IsTooOld(id ID) (isTooOld bool) {
	m.RLock()
	defer m.RUnlock()

	return id.Index() <= m.maxEvictedEpoch && !m.isRootBlock(id)
}

func (m *State[ID]) IsRootBlock(id ID) (isRootBlock bool) {
	m.RLock()
	defer m.RUnlock()

	return m.isRootBlock(id)
}

func (m *State[ID]) RootBlocks() (rootBlocks *set.AdvancedSet[ID]) {
	m.RLock()
	defer m.RUnlock()

	return m.rootBlocks.Clone()
}

func (m *State[ID]) MaxEvictedEpoch() epoch.Index {
	m.RLock()
	defer m.RUnlock()

	return m.maxEvictedEpoch
}

func (m *State[ID]) isRootBlock(id ID) (isRootBlock bool) {
	return m.rootBlocks.Has(id)
}

// setEvictedEpochAndUpdateRootBlocks atomically increases maxEvictedEpoch and updates the root blocks for the epoch.
func (m *State[ID]) setEvictedEpochAndUpdateRootBlocks(index epoch.Index, rootBlocks *set.AdvancedSet[ID]) (old epoch.Index) {
	m.Lock()
	defer m.Unlock()

	if old = m.maxEvictedEpoch; old >= index {
		return
	}

	m.maxEvictedEpoch = index
	m.rootBlocks = rootBlocks

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LockableManager //////////////////////////////////////////////////////////////////////////////////////////////

// LockableManager is a wrapper around the Manager that contains an additional Mutex used to synchronize the eviction
// process in the individual components.
type LockableManager[ID epoch.IndexedID] struct {
	// Manager is the underlying Manager.
	*State[ID]

	// RWMutex is the mutex that is used to synchronize the eviction process.
	sync.RWMutex
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
