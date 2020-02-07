package ed25119

import (
	"errors"

	"github.com/oasislabs/ed25519"
)

type PublicKey [PublicKeySize]byte

func (publicKey PublicKey) VerifySignature(data []byte, signature Signature) bool {
	return ed25519.Verify(publicKey[:], data, signature[:])
}

func (publicKey *PublicKey) UnmarshalBinary(bytes []byte) (err error) {
	if len(bytes) < PublicKeySize {
		return errors.New("not enough bytes")
	}

	copy(publicKey[:], bytes[:])

	return
}

const PublicKeySize = 32
