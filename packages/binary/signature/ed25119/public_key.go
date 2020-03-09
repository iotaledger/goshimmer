package ed25119

import (
	"errors"
	"fmt"

	"github.com/oasislabs/ed25519"
)

type PublicKey [PublicKeySize]byte

func PublicKeyFromBytes(bytes []byte) (result PublicKey, err error, consumedBytes int) {
	if len(bytes) < PublicKeySize {
		err = fmt.Errorf("bytes too short")

		return
	}

	copy(result[:], bytes)

	consumedBytes = PublicKeySize

	return
}

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
