package epoch

import (
	"sync"

	"github.com/iotaledger/hive.go/core/byteutils"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	ID CommitmentID

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

func (c *Commitment) publishChain(chain *CommitmentChain) (wasPublished bool) {
	c.Lock()
	defer c.Unlock()

	if wasPublished = c.chain == nil; wasPublished {
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

func NewCommitmentID(ei epoch.Index, ecr epoch.ECR, prevEC epoch.EC) (ec CommitmentID) {
	return blake2b.Sum256(byteutils.ConcatBytes(ei.Bytes(), ecr.Bytes(), prevEC.Bytes()))
}
