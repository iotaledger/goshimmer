package chainmanager

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/objectstorage/generic/model"
)

type Commitment struct {
	model.Storable[commitment.ID, Commitment, *Commitment, commitmentModel] `serix:"0"`

	solid       bool
	mainChildID commitment.ID
	children    map[commitment.ID]*Commitment
	chain       *Chain
}

func NewCommitment(id commitment.ID) (newCommitment *Commitment) {
	newCommitment = model.NewStorable[commitment.ID, Commitment](&commitmentModel{})
	newCommitment.children = make(map[commitment.ID]*Commitment)

	newCommitment.SetID(id)

	return newCommitment
}

type commitmentModel struct {
	Commitment *commitment.Commitment `serix:"0,optional"`
	Roots      *commitment.Roots      `serix:"1,optional"`
}

// FromObjectStorage deserializes a model from the object storage.
// TODO: this is just a temporary fix
func (c *Commitment) FromObjectStorage(key, data []byte) (err error) {
	if _, err = c.FromBytes(data); err != nil {
		return errors.Wrap(err, "failed to decode Model")
	}
	c.SetID(c.Commitment().ID())

	return nil
}

// ObjectStorageKey returns the bytes, that are used as a key to store the object in the k/v store.
// TODO: this is just a temporary fix
func (c *Commitment) ObjectStorageKey() (key []byte) {
	return c.Commitment().Index().Bytes()
}

func (c *Commitment) Commitment() (commitment *commitment.Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Commitment
}

func (c *Commitment) Children() []*Commitment {
	c.RLock()
	defer c.RUnlock()

	return lo.Values(c.children)
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
		c.InvalidateBytesCache()
	}

	return
}

func (c *Commitment) PublishRoots(roots *commitment.Roots) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.M.Roots == nil; published {
		c.M.Roots = roots
		c.InvalidateBytesCache()
	}

	return
}

func (c *Commitment) registerChild(child *Commitment) (isSolid bool, chain *Chain, wasForked bool) {
	c.Lock()
	defer c.Unlock()

	if len(c.children) == 0 {
		c.mainChildID = child.ID()
	}

	if c.children[child.ID()] = child; len(c.children) > 1 {
		return c.solid, NewChain(child), true
	}

	return c.solid, c.chain, false
}

func (c *Commitment) deleteChild(child *Commitment) {
	c.Lock()
	defer c.Unlock()

	delete(c.children, child.ID())
}

func (c *Commitment) mainChild() *Commitment {
	c.RLock()
	defer c.RUnlock()

	return c.children[c.mainChildID]
}

func (c *Commitment) setMainChild(commitment *Commitment) error {
	c.Lock()
	defer c.Unlock()

	if _, has := c.children[commitment.ID()]; !has {
		return errors.Errorf("trying to set a main child %s before registering it as a child", commitment.ID())
	}
	c.mainChildID = commitment.ID()

	return nil
}

func (c *Commitment) publishChain(chain *Chain) (wasPublished bool) {
	c.Lock()
	defer c.Unlock()

	if wasPublished = c.chain == nil; wasPublished {
		c.chain = chain
	}

	return
}

func (c *Commitment) replaceChain(chain *Chain) {
	c.Lock()
	defer c.Unlock()

	c.chain = chain
}
