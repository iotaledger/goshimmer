package id

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"golang.org/x/crypto/ed25519"
)

var errInvalidPubKeyLen = errors.New("id: invalid public key length")

// Identity offeres IDs in string/byte form and functions to check signatures.
type Identity struct {
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
	return &Identity{
		PublicKey: publicKey,
	}, nil
}

// FromIDKey creates a new id based on the ID in string form.
func FromIDKey(id string) (*Identity, error) {
	return NewIdentity([]byte(id))
}

// Returns the ID of the identity.
func (id *Identity) ID() []byte {
	return id.PublicKey
}

// Returns the ID of the identity as a string.
func (id *Identity) IDKey() string {
	return string(id.ID())
}

// String gives a human-readable version of the ID by hashing and returning
// the first bytes.
func (id *Identity) String() string {
	hash := sha256.Sum256(id.ID())
	return fmt.Sprintf("%X", hash[:8])
}

// Equal returns a boolean reporting whether the provided identity correspond to the same public key.
func (id *Identity) Equal(x *Identity) bool {
	return id == x || bytes.Equal(id.PublicKey, x.PublicKey)
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
