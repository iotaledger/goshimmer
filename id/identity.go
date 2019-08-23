package id

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"golang.org/x/crypto/ed25519"
)

var errInvalidPubKeyLen = errors.New("id: invalid public key length")

// Identity offeres IDs in string/byte form and functions to check signatures.
type Identity struct {
	ID       []byte
	StringID string

	PublicKey []byte
}

// Private is an Identity plus a private key for signature generation.
type Private struct {
	Identity

	privateKey []byte
}

// NewIdentity creates a new id based on the given public key.
func NewIdentity(publicKey []byte) (*Identity, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, errInvalidPubKeyLen
	}
	// the identifier is the hash of the public key
	id := sha256.Sum256(publicKey)

	return &Identity{
		ID:        id[:],
		StringID:  fmt.Sprintf("%x", id[:8]),
		PublicKey: publicKey,
	}, nil
}

// VerifySignature checks whether the data contains a valid signature of the message.
func (id *Identity) VerifySignature(msg, sig []byte) bool {
	return ed25519.Verify(id.PublicKey, msg, sig)
}

func newPrivate(publicKey, privateKey []byte) *Private {
	id, err := NewIdentity(publicKey)
	if err != nil {
		panic("id: failed creating id: " + err.Error())
	}

	return &Private{
		Identity:   *id,
		privateKey: privateKey,
	}
}

// GeneratePrivate creates a id based on a newly generated
// public/private key pair. It will panic if no such pair could be generated.
func GeneratePrivate() *Private {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic("id: failed generating key: " + err.Error())
	}

	return newPrivate(publicKey, privateKey)
}

// Sign signs the message with privateKey and returns the message plus the signature.
func (id *Private) Sign(msg []byte) []byte {
	return ed25519.Sign(id.privateKey, msg)
}
