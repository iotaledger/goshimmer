package commitment

import (
	"github.com/iotaledger/hive.go/core/byteutils"
	"golang.org/x/crypto/blake2b"
)

type RootsID = MerkleRoot

func NewRootsID(tangleRoot, stateMutationRoot, stateRoot, manaRoot MerkleRoot) RootsID {
	branch1Hashed := blake2b.Sum256(byteutils.ConcatBytes(tangleRoot.Bytes(), stateMutationRoot.Bytes()))
	branch2Hashed := blake2b.Sum256(byteutils.ConcatBytes(stateRoot.Bytes(), manaRoot.Bytes()))
	rootHashed := blake2b.Sum256(byteutils.ConcatBytes(branch1Hashed[:], branch2Hashed[:]))

	return NewMerkleRoot(rootHashed[:])
}

// Roots contains roots of trees of an epoch.
type Roots struct {
	ID RootsID

	TangleRoot        MerkleRoot `serix:"0"`
	StateMutationRoot MerkleRoot `serix:"1"`
	StateRoot         MerkleRoot `serix:"2"`
	ManaRoot          MerkleRoot `serix:"3"`
}

func NewRoots(id RootsID) (roots *Roots) {
	return &Roots{
		ID: id,
	}
}
