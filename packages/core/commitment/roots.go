package commitment

import (
	"github.com/iotaledger/hive.go/core/generics/model"
)

type Roots struct {
	model.Storable[ID, Roots, *Roots, roots] `serix:"0"`
}

type roots struct {
	TangleRoot        MerkleRoot `serix:"0"`
	StateMutationRoot MerkleRoot `serix:"1"`
	StateRoot         MerkleRoot `serix:"2"`
	ManaRoot          MerkleRoot `serix:"3"`
}

func NewRoots(id RootsID) (newRoots *Roots) {
	newRoots = model.NewStorable[RootsID, Roots](&roots{})
	newRoots.SetID(id)

	return newRoots
}

func (r *Roots) TangleRoot() (tangleRoot MerkleRoot) {
	r.RLock()
	defer r.RUnlock()

	return r.M.TangleRoot
}

func (r *Roots) SetTangleRoot(tangleRoot MerkleRoot) {
	r.Lock()
	defer r.Unlock()

	r.M.TangleRoot = tangleRoot

	r.SetModified()
}

func (r *Roots) StateMutationRoot() (stateMutationRoot MerkleRoot) {
	r.RLock()
	defer r.RUnlock()

	return r.M.StateMutationRoot
}

func (r *Roots) SetStateMutationRoot(stateMutationRoot MerkleRoot) {
	r.Lock()
	defer r.Unlock()

	r.M.StateMutationRoot = stateMutationRoot

	r.SetModified()
}

func (r *Roots) StateRoot() (stateRoot MerkleRoot) {
	r.RLock()
	defer r.RUnlock()

	return r.M.StateRoot
}

func (r *Roots) SetStateRoot(stateRoot MerkleRoot) {
	r.Lock()
	defer r.Unlock()

	r.M.StateRoot = stateRoot

	r.SetModified()
}

func (r *Roots) ManaRoot() (manaRoot MerkleRoot) {
	r.RLock()
	defer r.RUnlock()

	return r.M.ManaRoot
}

func (r *Roots) SetManaRoot(manaRoot MerkleRoot) {
	r.Lock()
	defer r.Unlock()

	r.M.ManaRoot = manaRoot

	r.SetModified()
}
