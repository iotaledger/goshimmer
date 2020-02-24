package ed25119

import (
	"github.com/oasislabs/ed25519"
)

type PrivateKey [PrivateKeySize]byte

func (privateKey PrivateKey) Sign(data []byte) (result Signature) {
	copy(result[:], ed25519.Sign(privateKey[:], data))

	return
}

const PrivateKeySize = 64
