package chainmanager

import (
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	model.Storable[commitment.ID, Commitment, *Commitment, *commitment.Commitment] `serix:"0"`

	children    []*Commitment
	chain       *Chain
	entityMutex *syncutils.StarvingMutex
}

func NewCommitment(id commitment.ID) (newCommitment *Commitment) {
	newCommitment = model.NewStorable[commitment.ID, Commitment, *commitment.Commitment](nil)
	newCommitment.SetID(id)
	newCommitment.children = make([]*Commitment, 0)
	newCommitment.entityMutex = syncutils.NewStarvingMutex()

	return newCommitment
}

func (c *Commitment) Commitment() (commitment *commitment.Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.M
}

func (c *Commitment) PrevID() (prevID commitment.ID) {
	c.RLock()
	defer c.RUnlock()

	if c.M == nil {
		return
	}

	return c.M.PrevID()
}

func (c *Commitment) Index() (index epoch.Index) {
	c.RLock()
	defer c.RUnlock()

	if c.M == nil {
		return
	}

	return c.M.Index()
}

func (c *Commitment) RootsID() (rootsID commitment.ID) {
	c.RLock()
	defer c.RUnlock()

	if c.M == nil {
		return
	}

	return c.M.RootsID()
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

func (c *Commitment) PublishData(commitment *commitment.Commitment) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.M == nil; published {
		c.M = commitment
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

func (c *Commitment) PublishRoots(root commitment.MerkleRoot, root2 commitment.MerkleRoot, root3 commitment.MerkleRoot, root4 commitment.MerkleRoot) {

}
