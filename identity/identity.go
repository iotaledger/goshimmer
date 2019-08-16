package identity

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"golang.org/x/crypto/ed25519"
)

var (
	ErrInvalidPubKeyLen = errors.New("identity: invalid public key length")
)

type Identity struct {
	Id       []byte
	StringId string

	PublicKey []byte
}

type PrivateIdentity struct {
	Identity

	privateKey []byte
}

// Creates a new identity based on the given public key.
func NewIdentity(publicKey []byte) (*Identity, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, ErrInvalidPubKeyLen
	}
	// the identifier is the hash of the public key
	identifier := sha256.Sum224(publicKey)

	return &Identity{
		Id:        identifier[:],
		StringId:  fmt.Sprintf("%x", identifier),
		PublicKey: append([]byte(nil), publicKey...),
	}, nil
}

// Verifies whether the data contains a valid signature of the message.
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

// Generates a identity based on a newly generated public/private key pair.
// It will panic if no such pair could be generated.
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
