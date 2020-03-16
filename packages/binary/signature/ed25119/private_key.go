package ed25119

import (
	"fmt"

	"github.com/oasislabs/ed25519"
)

type PrivateKey [PrivateKeySize]byte

func PrivateKeyFromBytes(bytes []byte) (result PrivateKey, err error, consumedBytes int) {
	if len(bytes) < PrivateKeySize {
		err = fmt.Errorf("bytes too short")

		return
	}

	copy(result[:], bytes)

	consumedBytes = PrivateKeySize

	return
}

func (privateKey PrivateKey) Sign(data []byte) (result Signature) {
	copy(result[:], ed25519.Sign(privateKey[:], data))

	return
}

const PrivateKeySize = 64
