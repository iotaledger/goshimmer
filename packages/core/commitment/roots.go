package commitment

import (
	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/generics/model"
	"golang.org/x/crypto/blake2b"
)

type Roots struct {
	model.Immutable[Roots, *Roots, roots] `serix:"0"`
}

type roots struct {
	TangleRoot        MerkleRoot `serix:"0"`
	StateMutationRoot MerkleRoot `serix:"1"`
	StateRoot         MerkleRoot `serix:"2"`
	ManaRoot          MerkleRoot `serix:"3"`
}

func NewRoots(tangleRoot, stateMutationRoot, stateRoot, manaRoot MerkleRoot) (newRoots *Roots) {
	return model.NewImmutable[Roots](&roots{
		TangleRoot:        tangleRoot,
		StateMutationRoot: stateMutationRoot,
		StateRoot:         stateRoot,
		ManaRoot:          manaRoot,
	})
}

func (r *Roots) ID() (id RootsID) {
	branch1Hashed := blake2b.Sum256(byteutils.ConcatBytes(r.M.TangleRoot.Bytes(), r.M.StateMutationRoot.Bytes()))
	branch2Hashed := blake2b.Sum256(byteutils.ConcatBytes(r.M.StateRoot.Bytes(), r.M.ManaRoot.Bytes()))
	rootHashed := blake2b.Sum256(byteutils.ConcatBytes(branch1Hashed[:], branch2Hashed[:]))

	return NewMerkleRoot(rootHashed[:])
}

func (r *Roots) TangleRoot() (tangleRoot MerkleRoot) {
	return r.M.TangleRoot
}

func (r *Roots) StateMutationRoot() (stateMutationRoot MerkleRoot) {
	return r.M.StateMutationRoot
}

func (r *Roots) StateRoot() (stateRoot MerkleRoot) {
	return r.M.StateRoot
}

func (r *Roots) ManaRoot() (manaRoot MerkleRoot) {
	return r.M.ManaRoot
}
