package commitment

import (
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

type MerkleRoot [blake2b.Size256]byte

var EmptyMerkleRoot = MerkleRoot{}

func NewMerkleRoot(bytes []byte) MerkleRoot {
	b := [blake2b.Size256]byte{}
	copy(b[:], bytes[:])
	return b
}

func (m MerkleRoot) Base58() string {
	return base58.Encode(m[:])
}

func (m MerkleRoot) Bytes() []byte {
	return m[:]
}
