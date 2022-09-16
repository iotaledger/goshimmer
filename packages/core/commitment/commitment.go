package commitment

import (
	"github.com/iotaledger/hive.go/core/generics/model"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	model.Storable[ID, Commitment, *Commitment, commitment] `serix:"0"`
}

type commitment struct {
	Prev  ID          `serix:"0"`
	Index epoch.Index `serix:"1"`
	Roots *Roots      `serix:"2"`
}

func New(id ID) (newCommitment *Commitment) {
	newCommitment = model.NewStorable[ID, Commitment](&commitment{})
	newCommitment.SetID(id)

	return newCommitment
}

func (c *Commitment) Index() (ei epoch.Index) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Index
}

func (c *Commitment) RootsID() (ecr RootsID) {
	c.RLock()
	defer c.RUnlock()

	if model.NewStorable[ID, Commitment](&commitment{}) == nil {
		return
	}

	return c.M.Roots.ID
}

func (c *Commitment) Roots() (roots *Roots) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Roots
}

func (c *Commitment) PrevID() (prevEC ID) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Prev
}

func (c *Commitment) PublishData(index epoch.Index, ecr RootsID, previousEC ID) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.M.Roots == nil; published {
		c.M.Prev = previousEC
		c.M.Index = index
		c.M.Roots = NewRoots(ecr)
	}

	return
}

func (c *Commitment) PublishRoots(tangleRoot MerkleRoot, stateMutationRoot MerkleRoot, stateRoot MerkleRoot, manaRoot MerkleRoot) (published bool) {
	c.Lock()
	defer c.Unlock()

	if c.M.Roots != nil && c.M.Roots.TangleRoot == tangleRoot && c.M.Roots.StateMutationRoot == stateMutationRoot && c.M.Roots.StateRoot == stateRoot && c.M.Roots.ManaRoot == manaRoot {
		return false
	}

	if c.M.Roots == nil {
		c.M.Roots = NewRoots(NewRootsID(tangleRoot, stateMutationRoot, stateRoot, manaRoot))
	}

	c.M.Roots.TangleRoot = tangleRoot
	c.M.Roots.StateMutationRoot = stateMutationRoot
	c.M.Roots.StateRoot = stateRoot
	c.M.Roots.ManaRoot = manaRoot

	return true
}
