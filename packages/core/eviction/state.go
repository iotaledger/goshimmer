package eviction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

type State struct {
	maxDroppedEpoch   epoch.Index
	isRootBlock       func(models.BlockID) *models.Block
	evictionCallbacks []func(index epoch.Index)

	pruningMutex sync.RWMutex
}

func NewState(rootBlockProvider func(models.BlockID) *models.Block) *State {
	return &State{
		isRootBlock: rootBlockProvider,
	}
}

func (s *State) Evict(epochIndex epoch.Index) {
	s.pruningMutex.Lock()
	defer s.pruningMutex.Unlock()

	for s.maxDroppedEpoch < epochIndex {
		s.maxDroppedEpoch++
		for _, evictionCallback := range s.evictionCallbacks {
			evictionCallback(s.maxDroppedEpoch)
		}
	}
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (s *State) IsTooOld(id models.BlockID) (isTooOld bool) {
	return id.EpochIndex <= s.maxDroppedEpoch && s.isRootBlock(id) == nil
}

func (s *State) RLock() {
	s.pruningMutex.RLock()
}

func (s *State) RUnlock() {
	s.pruningMutex.RUnlock()
}
