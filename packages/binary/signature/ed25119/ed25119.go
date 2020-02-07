package ed25119

import (
	"crypto/rand"

	"github.com/oasislabs/ed25519"
)

func GenerateKeyPair() (keyPair KeyPair) {
	if public, private, err := ed25519.GenerateKey(rand.Reader); err != nil {
		panic(err)
	} else {
		copy(keyPair.PrivateKey[:], private)
		copy(keyPair.PublicKey[:], public)

		return
	}
}
