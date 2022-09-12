package epoch

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region CommitmentChain //////////////////////////////////////////////////////////////////////////////////////////////

type CommitmentChain struct {
	ForkingPoint *Commitment

	commitmentsByIndex map[epoch.Index]*Commitment

	sync.RWMutex
}

func NewCommitmentChain(forkingPoint *Commitment) (fork *CommitmentChain) {
	return &CommitmentChain{
		ForkingPoint: forkingPoint,

		commitmentsByIndex: map[epoch.Index]*Commitment{
			forkingPoint.EI(): forkingPoint,
		},
	}
}

func (c *CommitmentChain) Commitment(index epoch.Index) (commitment *Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.commitmentsByIndex[index]
}

func (c *CommitmentChain) addCommitment(commitment *Commitment) {
	c.Lock()
	defer c.Unlock()

	c.commitmentsByIndex[commitment.EI()] = commitment
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
