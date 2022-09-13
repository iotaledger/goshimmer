package commitmentmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	EC     epoch.EC
	ei     *epoch.Index
	ecr    *epoch.ECR
	prevEC *epoch.EC

	children    []*Commitment
	chain       *Chain
	entityMutex *syncutils.StarvingMutex

	sync.RWMutex
}

func NewCommitment(ec epoch.EC) (commitment *Commitment) {
	return &Commitment{
		EC:          ec,
		children:    make([]*Commitment, 0),
		entityMutex: syncutils.NewStarvingMutex(),
	}
}

func (c *Commitment) EI() (ei epoch.Index) {
	c.RLock()
	defer c.RUnlock()

	if c.ei == nil {
		return
	}

	return *c.ei
}

func (c *Commitment) ECR() (ecr epoch.ECR) {
	c.RLock()
	defer c.RUnlock()

	if c.ecr == nil {
		return
	}

	return *c.ecr
}

func (c *Commitment) PrevEC() (prevEC epoch.EC) {
	c.RLock()
	defer c.RUnlock()

	if c.prevEC == nil {
		return
	}

	return *c.prevEC
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

func (c *Commitment) LockDAGEntity() {
	c.entityMutex.Lock()
}

func (c *Commitment) UnlockDAGEntity() {
	c.entityMutex.Unlock()
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

func (c *Commitment) publishECRecord(index epoch.Index, ecr epoch.ECR, previousEC epoch.EC) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.ei == nil; published {
		c.ei = &index
		c.ecr = &ecr
		c.prevEC = &previousEC
	}

	return
}
