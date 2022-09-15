package chain

import (
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Commitment struct {
	children    []*Commitment
	chain       *Chain
	entityMutex *syncutils.StarvingMutex

	*commitment.Commitment
}

func NewCommitment(id commitment.ID) (newCommitment *Commitment) {
	return &Commitment{
		children:    make([]*Commitment, 0),
		entityMutex: syncutils.NewStarvingMutex(),

		Commitment: commitment.New(id),
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

func (c *Commitment) lockEntity() {
	c.entityMutex.Lock()
}

func (c *Commitment) unlockEntity() {
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
