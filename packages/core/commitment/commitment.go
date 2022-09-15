package commitment

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	ID ID

	index  *epoch.Index
	prevID *ID
	roots  *Roots

	sync.RWMutex
	objectstorage.StorableObject
}

func New(id ID) (commitment *Commitment) {
	return &Commitment{
		ID: id,
	}
}

func (c *Commitment) Index() (ei epoch.Index) {
	c.RLock()
	defer c.RUnlock()

	if c.index == nil {
		return
	}

	return *c.index
}

func (c *Commitment) RootsID() (ecr RootsID) {
	c.RLock()
	defer c.RUnlock()

	if c.roots == nil {
		return
	}

	return c.roots.ID
}

func (c *Commitment) Roots() (roots *Roots) {
	c.RLock()
	defer c.RUnlock()

	return c.roots
}

func (c *Commitment) PrevID() (prevEC ID) {
	c.RLock()
	defer c.RUnlock()

	if c.prevID == nil {
		return
	}

	return *c.prevID
}

func (c *Commitment) PublishData(index epoch.Index, ecr RootsID, previousEC ID) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.index == nil; published {
		c.index = &index
		c.roots = NewRoots(ecr)
		c.prevID = &previousEC
	}

	return
}

func (c *Commitment) PublishRoots(tangleRoot MerkleRoot, stateMutationRoot MerkleRoot, stateRoot MerkleRoot, manaRoot MerkleRoot) (published bool) {
	c.Lock()
	defer c.Unlock()

	if c.roots != nil && c.roots.TangleRoot == tangleRoot && c.roots.StateMutationRoot == stateMutationRoot && c.roots.StateRoot == stateRoot && c.roots.ManaRoot == manaRoot {
		return false
	}

	if c.roots == nil {
		c.roots = NewRoots(NewRootsID(tangleRoot, stateMutationRoot, stateRoot, manaRoot))
	}

	c.roots.TangleRoot = tangleRoot
	c.roots.StateMutationRoot = stateMutationRoot
	c.roots.StateRoot = stateRoot
	c.roots.ManaRoot = manaRoot

	return true
}
