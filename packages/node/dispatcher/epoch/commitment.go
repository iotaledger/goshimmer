package epoch

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	ID CommitmentID

	prev     *Commitment
	children []*Commitment
	chain    *CommitmentChain

	*epoch.ECRecord
	sync.RWMutex
}

func NewCommitment(id CommitmentID) (commitment *Commitment) {
	return &Commitment{
		ID:       id,
		children: make([]*Commitment, 0),
	}
}

func (c *Commitment) Children() (children []*Commitment) {
	c.RLock()
	defer c.RUnlock()

	children = make([]*Commitment, len(c.children))
	copy(children, c.children)

	return
}

func (c *Commitment) Chain() (chain *CommitmentChain) {
	c.RLock()
	defer c.RUnlock()

	return c.chain
}

func (c *Commitment) registerChild(child *Commitment) (chain *CommitmentChain) {
	c.Lock()
	defer c.Unlock()

	if c.children = append(c.children, child); len(c.children) > 1 {
		return NewCommitmentChain(child)
	}

	return c.chain
}

func (c *Commitment) setChain(chain *CommitmentChain) (updated bool) {
	c.Lock()
	defer c.Unlock()

	if updated = c.chain != chain; updated {
		c.chain = chain
	}

	return
}

func (c *Commitment) publishECRecord(index epoch.Index, commitmentRoot epoch.ECR, previousEC epoch.EC) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.ECRecord == nil; published {
		c.ECRecord = epoch.NewECRecord(index)
		c.ECRecord.SetPrevEC(previousEC)
		c.ECRecord.SetECR(commitmentRoot)
	}

	return
}

type CommitmentID = epoch.EC
