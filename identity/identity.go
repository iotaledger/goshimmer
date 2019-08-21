package identity

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"golang.org/x/crypto/ed25519"
)

var errInvalidPubKeyLen = errors.New("identity: invalid public key length")

// Identity offeres IDs in string/byte form and functions to check signatures.
type Identity struct {
	ID       []byte
	StringID string

	PublicKey []byte
}

// PrivateIdentity is an Identiy plus a private key for signature generation.
type PrivateIdentity struct {
	Identity

	privateKey []byte
}

// NewIdentity creates a new identity based on the given public key.
func NewIdentity(publicKey []byte) (*Identity, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, errInvalidPubKeyLen
	}
	// the identifier is the hash of the public key
	identifier := sha256.Sum224(publicKey)

	return &Identity{
		ID:        identifier[:],
		StringID:  fmt.Sprintf("%x", identifier),
		PublicKey: append([]byte(nil), publicKey...),
	}, nil
}

// VerifySignature checks whether the data contains a valid signature of the message.
func (id *Identity) VerifySignature(msg, sig []byte) bool {
	return ed25519.Verify(id.PublicKey, msg, sig)
}

func newPrivateIdentity(publicKey, privateKey []byte) *PrivateIdentity {
	identity, err := NewIdentity(publicKey)
	if err != nil {
		panic("identity: failed creating identity: " + err.Error())
	}

	return &PrivateIdentity{
		Identity:   *identity,
		privateKey: privateKey,
	}
}

// GeneratePrivateIdentity creates a identity based on a newly generated
// public/private key pair. It will panic if no such pair could be generated.
func GeneratePrivateIdentity() *PrivateIdentity {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic("identity: failed generating key: " + err.Error())
	}

	return newPrivateIdentity(publicKey, privateKey)
}

// Sign signs the message with privateKey and returns the message plus the signature.
func (id *PrivateIdentity) Sign(msg []byte) []byte {
	return ed25519.Sign(id.privateKey, msg)
}
