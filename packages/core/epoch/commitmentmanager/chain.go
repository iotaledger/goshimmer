package commitmentmanager

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Chain struct {
	ForkingPoint *Commitment

	commitmentsByIndex map[epoch.Index]*Commitment

	sync.RWMutex
}

func NewChain(forkingPoint *Commitment) (fork *Chain) {
	return &Chain{
		ForkingPoint: forkingPoint,

		commitmentsByIndex: map[epoch.Index]*Commitment{
			forkingPoint.EI(): forkingPoint,
		},
	}
}

func (c *Chain) Commitment(index epoch.Index) (commitment *Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.commitmentsByIndex[index]
}

func (c *Chain) Size() int {
	c.RLock()
	defer c.RUnlock()

	return len(c.commitmentsByIndex)
}

func (c *Chain) addCommitment(commitment *Commitment) {
	c.Lock()
	defer c.Unlock()

	c.commitmentsByIndex[commitment.EI()] = commitment
}
