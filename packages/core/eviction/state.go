package eviction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type State[ID epoch.IndexedID] struct {
	Events *Events

	maxEvictedEpoch epoch.Index
	isRootBlockFunc func(ID) bool

	sync.RWMutex
}

func NewState[ID epoch.IndexedID](isRootBlockFunc func(ID) bool) (newState *State[ID]) {
	return &State[ID]{
		Events:          NewEvents(),
		maxEvictedEpoch: 0,
		isRootBlockFunc: isRootBlockFunc,
	}
}

// Lockable returns a lockable version of the Manager that contains an additional mutex used to synchronize the eviction
// process inside the components.
func (m *State[ID]) Lockable() (newLockableState *LockableState[ID]) {
	return &LockableState[ID]{
		State: m,
	}
}

func (m *State[ID]) EvictUntil(epochIndex epoch.Index) {
	m.Lock()
	defer m.Unlock()

	previousEvicted := m.maxEvictedEpoch
	for currentIndex := previousEvicted + 1; currentIndex <= epochIndex; currentIndex++ {
		m.Events.EpochEvicted.Trigger(currentIndex)
	}

	m.maxEvictedEpoch = epochIndex
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (m *State[ID]) IsTooOld(id ID) (isTooOld bool) {
	m.RLock()
	defer m.RUnlock()

	return id.Index() <= m.maxEvictedEpoch && !m.isRootBlockFunc(id)
}

func (m *State[ID]) IsRootBlock(id ID) (isRootBlock bool) {
	m.RLock()
	defer m.RUnlock()

	return m.isRootBlockFunc(id)
}

func (m *State[ID]) MaxEvictedEpoch() epoch.Index {
	m.RLock()
	defer m.RUnlock()

	return m.maxEvictedEpoch
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LockableState ////////////////////////////////////////////////////////////////////////////////////////////////

// LockableState is a wrapper around the Manager that contains an additional Mutex used to synchronize the eviction
// process in the individual components.
type LockableState[ID epoch.IndexedID] struct {
	// Manager is the underlying Manager.
	*State[ID]

	// RWMutex is the mutex that is used to synchronize the eviction process.
	sync.RWMutex
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
