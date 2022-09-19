package commitment

import (
	"github.com/iotaledger/hive.go/core/generics/model"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Commitment struct {
	model.Storable[ID, Commitment, *Commitment, commitment] `serix:"0"`
}

type commitment struct {
	PrevID ID          `serix:"0"`
	Index  epoch.Index `serix:"1"`
	Roots  *Roots      `serix:"2"`
}

func New(id ID) (newCommitment *Commitment) {
	newCommitment = model.NewStorable[ID, Commitment](&commitment{})
	newCommitment.SetID(id)

	return newCommitment
}

func (c *Commitment) PrevID() (prevID ID) {
	c.RLock()
	defer c.RUnlock()

	return c.M.PrevID
}

func (c *Commitment) Index() (index epoch.Index) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Index
}

func (c *Commitment) RootsID() (rootsID RootsID) {
	c.RLock()
	defer c.RUnlock()

	if c.M.Roots == nil {
		return
	}

	return c.M.Roots.ID()
}

func (c *Commitment) Roots() (roots *Roots) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Roots
}

func (c *Commitment) PublishData(prevID ID, index epoch.Index, rootsID RootsID) (published bool) {
	c.Lock()
	defer c.Unlock()

	if published = c.M.Roots == nil; published {
		c.M.PrevID = prevID
		c.M.Index = index
		c.M.Roots = NewRoots(rootsID)
	}

	return
}

func (c *Commitment) PublishRoots(tangleRoot MerkleRoot, mutationRoot MerkleRoot, stateRoot MerkleRoot, manaRoot MerkleRoot) (published bool) {
	c.Lock()
	defer c.Unlock()

	if c.M.Roots != nil && c.M.Roots.TangleRoot() == tangleRoot && c.M.Roots.StateMutationRoot() == mutationRoot && c.M.Roots.StateRoot() == stateRoot && c.M.Roots.ManaRoot() == manaRoot {
		return false
	}

	if c.M.Roots == nil {
		c.M.Roots = NewRoots(NewRootsID(tangleRoot, mutationRoot, stateRoot, manaRoot))
	}

	c.M.Roots.SetTangleRoot(tangleRoot)
	c.M.Roots.SetStateMutationRoot(mutationRoot)
	c.M.Roots.SetStateRoot(stateRoot)
	c.M.Roots.SetManaRoot(manaRoot)

	return true
}
