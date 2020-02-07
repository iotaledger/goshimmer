package identity

import (
	"crypto/rand"

	"github.com/oasislabs/ed25519"
)

type Identity struct {
	Type       Type
	PublicKey  []byte
	PrivateKey []byte
}

func New(publicKey []byte, optionalPrivateKey ...[]byte) *Identity {
	this := &Identity{
		PublicKey: make([]byte, len(publicKey)),
	}

	copy(this.PublicKey, publicKey)

	if len(optionalPrivateKey) == 0 {
		this.Type = Public
	} else {
		this.Type = Private
		this.PrivateKey = optionalPrivateKey[0]
	}

	return this
}

func Generate() *Identity {
	if public, private, err := ed25519.GenerateKey(rand.Reader); err != nil {
		panic(err)
	} else {
		return New(public, private)
	}
}

func (identity *Identity) Sign(data []byte) (sig []byte) {
	sig = ed25519.Sign(identity.PrivateKey, data)

	return
}

func (identity *Identity) VerifySignature(data []byte, signature []byte) bool {
	return ed25519.Verify(identity.PublicKey, data, signature)
}
