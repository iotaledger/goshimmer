package chainmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/core/slot"
)

type Chain struct {
	ForkingPoint *Commitment

	latestCommitmentIndex slot.Index
	commitmentsByIndex    map[slot.Index]*Commitment

	sync.RWMutex
}

func NewChain(forkingPoint *Commitment) (fork *Chain) {
	forkingPointIndex := forkingPoint.Commitment().Index()

	return &Chain{
		ForkingPoint:          forkingPoint,
		latestCommitmentIndex: forkingPointIndex,
		commitmentsByIndex: map[slot.Index]*Commitment{
			forkingPointIndex: forkingPoint,
		},
	}
}

func (c *Chain) IsSolid() (isSolid bool) {
	c.RLock()
	defer c.RUnlock()

	return c.ForkingPoint.IsSolid()
}

func (c *Chain) Commitment(index slot.Index) (commitment *Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.commitmentsByIndex[index]
}

func (c *Chain) Size() int {
	c.RLock()
	defer c.RUnlock()

	return len(c.commitmentsByIndex)
}

func (c *Chain) LatestCommitment() *Commitment {
	c.RLock()
	defer c.RUnlock()

	return c.commitmentsByIndex[c.latestCommitmentIndex]
}

func (c *Chain) addCommitment(commitment *Commitment) {
	c.Lock()
	defer c.Unlock()

	commitmentIndex := commitment.Commitment().Index()
	if commitmentIndex > c.latestCommitmentIndex {
		c.latestCommitmentIndex = commitmentIndex
	}

	c.commitmentsByIndex[commitmentIndex] = commitment
}

func (c *Chain) dropCommitmentsAfter(index slot.Index) {
	c.Lock()
	defer c.Unlock()

	for i := index + 1; i <= c.latestCommitmentIndex; i++ {
		delete(c.commitmentsByIndex, i)
	}

	if index < c.latestCommitmentIndex {
		c.latestCommitmentIndex = index
	}
}
