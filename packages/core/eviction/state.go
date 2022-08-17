package eviction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

type State struct {
	maxDroppedEpoch   epoch.Index
	isRootBlock       func(models.BlockID) bool
	evictionCallbacks []func(index epoch.Index)

	pruningMutex sync.RWMutex
}

func NewState(rootBlockProvider func(models.BlockID) bool) *State {
	return &State{
		isRootBlock: rootBlockProvider,
	}
}

func (s *State) Evict(epochIndex epoch.Index) {
	s.pruningMutex.Lock()
	defer s.pruningMutex.Unlock()

	for s.maxDroppedEpoch < epochIndex {
		s.maxDroppedEpoch++
		// Components register from the innermost component first. However, we need to evict starting from the outermost
		// component to the inner.
		for i := len(s.evictionCallbacks) - 1; i >= 0; i-- {
			s.evictionCallbacks[i](s.maxDroppedEpoch)
		}
	}
}

func (s *State) RegisterEvictionCallback(callback func(index epoch.Index)) {
	s.pruningMutex.Lock()
	defer s.pruningMutex.Unlock()

	s.evictionCallbacks = append(s.evictionCallbacks, callback)
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (s *State) IsTooOld(id models.BlockID) (isTooOld bool) {
	return !s.isRootBlock(id) && id.EpochIndex <= s.maxDroppedEpoch
}

func (s *State) IsRootBlock(id models.BlockID) (rootBlock bool) {
	return s.isRootBlock(id)
}

func (s *State) RLock() {
	s.pruningMutex.RLock()
}

func (s *State) RUnlock() {
	s.pruningMutex.RUnlock()
}

func (s *State) MaxDroppedEpoch() epoch.Index {
	s.pruningMutex.RLock()
	defer s.pruningMutex.RUnlock()

	return s.maxDroppedEpoch
}
