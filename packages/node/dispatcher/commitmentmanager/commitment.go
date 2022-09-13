package commitmentmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/core/byteutils"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	EC epoch.EC

	children []*Commitment
	chain    *Chain

	*epoch.ECRecord
	sync.RWMutex
}

func NewCommitment(ec epoch.EC) (commitment *Commitment) {
	return &Commitment{
		EC:       ec,
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

func (c *Commitment) Chain() (chain *Chain) {
	c.RLock()
	defer c.RUnlock()

	return c.chain
}

func (c *Commitment) registerChild(child *Commitment) (chain *Chain, wasForked bool) {
	c.Lock()
	defer c.Unlock()

	if c.children = append(c.children, child); len(c.children) > 1 {
		return NewChain(child), true
	}

	return c.chain, false
}

func (c *Commitment) publishChain(chain *Chain) (wasPublished bool) {
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

func NewEC(ei epoch.Index, ecr epoch.ECR, prevEC epoch.EC) (ec epoch.EC) {
	return blake2b.Sum256(byteutils.ConcatBytes(ei.Bytes(), ecr.Bytes(), prevEC.Bytes()))
}
