package chainmanager

import (
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Commitment struct {
	model.Storable[commitment.ID, Commitment, *Commitment, commitmentModel] `serix:"0"`

	children    []*Commitment
	chain       *Chain
	entityMutex *syncutils.StarvingMutex
}

func NewCommitment(id commitment.ID) (newCommitment *Commitment) {
	newCommitment = model.NewStorable[commitment.ID, Commitment](&commitmentModel{})
	newCommitment.children = make([]*Commitment, 0)
	newCommitment.entityMutex = syncutils.NewStarvingMutex()

	newCommitment.SetID(id)

	return newCommitment
}

type commitmentModel struct {
	Commitment *commitment.Commitment `serix:"0,optional"`
	Roots      *commitment.Roots      `serix:"1,optional"`
}

func (c *Commitment) Commitment() (commitment *commitment.Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Commitment
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

func (c *Commitment) PublishCommitment(commitment *commitment.Commitment) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.M.Commitment == nil; published {
		c.M.Commitment = commitment
	}

	return
}

func (c *Commitment) PublishRoots(roots *commitment.Roots) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.M.Roots == nil; published {
		c.M.Roots = roots
	}

	return
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
