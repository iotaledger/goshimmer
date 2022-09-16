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
