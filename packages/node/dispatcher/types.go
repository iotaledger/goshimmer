package dispatcher

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region ForkManager //////////////////////////////////////////////////////////////////////////////////////////////////

type ForkManager struct {
	forksByEC map[epoch.EC]*EpochCommitmentChain

	sync.RWMutex
}

func NewForksManager() (forksManager *ForkManager) {
	return &ForkManager{
		forksByEC: make(map[epoch.EC]*EpochCommitmentChain),
	}
}

func (f *ForkManager) Fork(ec epoch.EC) (fork *EpochCommitmentChain) {
	f.RLock()
	defer f.RUnlock()

	fork, exists := f.forksByEC[ec]
	if !exists {
		return nil
	}

	return fork
}

func (f *ForkManager) AddMapping(ec epoch.EC, fork *EpochCommitmentChain) {
	f.Lock()
	defer f.Unlock()

	f.forksByEC[ec] = fork
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
