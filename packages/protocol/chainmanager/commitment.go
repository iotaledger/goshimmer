package chainmanager

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Commitment struct {
	model.Storable[commitment.ID, Commitment, *Commitment, commitmentModel] `serix:"0"`

	solid       bool
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

// TODO: this is just a temporary fix
// FromObjectStorage deserializes a model from the object storage.
func (c *Commitment) FromObjectStorage(key, data []byte) (err error) {
	if _, err = c.FromBytes(data); err != nil {
		return errors.Errorf("failed to decode Model: %w", err)
	}
	c.SetID(c.Commitment().ID())

	return nil
}

// TODO: this is just a temporary fix
// ObjectStorageKey returns the bytes, that are used as a key to store the object in the k/v store.
func (c *Commitment) ObjectStorageKey() (key []byte) {
	return c.Commitment().Index().Bytes()
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

func (c *Commitment) IsSolid() (isSolid bool) {
	c.RLock()
	defer c.RUnlock()

	return c.solid
}

func (c *Commitment) SetSolid(solid bool) (updated bool) {
	c.Lock()
	defer c.Unlock()

	if updated = c.solid != solid; updated {
		c.solid = solid
	}

	return
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

func (c *Commitment) registerChild(child *Commitment) (isSolid bool, chain *Chain, wasForked bool) {
	c.Lock()
	defer c.Unlock()

	if c.children = append(c.children, child); len(c.children) > 1 {
		return c.solid, NewChain(child), true
	}

	return c.solid, c.chain, false
}

func (c *Commitment) publishChain(chain *Chain) (wasPublished bool) {
	c.Lock()
	defer c.Unlock()

	if wasPublished = c.chain == nil; wasPublished {
		c.chain = chain
	}

	return
}
