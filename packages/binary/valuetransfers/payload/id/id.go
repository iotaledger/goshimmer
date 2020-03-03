package id

import (
	"github.com/mr-tron/base58"
)

type Id [Length]byte

func New(idBytes []byte) (result Id) {
	copy(result[:], idBytes)

	return
}

func (id Id) String() string {
	return base58.Encode(id[:])
}

var Empty Id

const Length = 32
