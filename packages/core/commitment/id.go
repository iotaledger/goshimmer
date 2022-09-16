package commitment

import (
	"github.com/iotaledger/hive.go/core/byteutils"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type ID = MerkleRoot

func NewID(ei epoch.Index, ecr RootsID, prevEC ID) (ec ID) {
	return blake2b.Sum256(byteutils.ConcatBytes(ei.Bytes(), ecr.Bytes(), prevEC.Bytes()))
}
