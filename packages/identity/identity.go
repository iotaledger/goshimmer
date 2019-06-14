package identity

import (
	"errors"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/crypto"
	"golang.org/x/crypto/ed25519"
)

type Identity struct {
	Identifier       []byte
	StringIdentifier string
	PublicKey        []byte
	privateKey       []byte
}

// Creates a new identity based on the given public key.
func NewPublicIdentity(publicKey []byte) *Identity {
	identifier := crypto.Hash20(publicKey)

	return &Identity{
		Identifier:       identifier,
		StringIdentifier: fmt.Sprintf("%x", identifier),
		PublicKey:        publicKey,
		privateKey:       nil,
	}
}

// Generates a identity based on a newly generated public/private key pair.
// It will panic if no such pair could be generated.
func GeneratePrivateIdentity() *Identity {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic("identity: failed generating key: " + err.Error())
	}

	return newPrivateIdentity(publicKey, privateKey)
}

// Sign signs the message with privateKey and returns the message plus the signature.
func (id *Identity) AddSignature(msg []byte) []byte {
	signatureStart := len(msg)

	signature := ed25519.Sign(id.privateKey, msg)

	data := make([]byte, signatureStart+SIGNATURE_BYTE_LENGTH)

	copy(data[:signatureStart], msg)

	// add public key and signature
	copy(data[signatureStart:signatureStart+ed25519.PublicKeySize], id.PublicKey)
	copy(data[signatureStart+ed25519.PublicKeySize:], signature)

	return data
}

// Verifies whether the data contains a valid signature of the message. It will
// panic if len(data) is not long enough to contain data and signature.
func (id *Identity) VerifySignature(data []byte) bool {
	signatureStart := len(data) - SIGNATURE_BYTE_LENGTH
	if signatureStart <= 0 {
		panic("identity: bad data length")
	}

	msg := data[:signatureStart]

	// ignore the public key
	sig := data[signatureStart+ed25519.PublicKeySize:]

	return ed25519.Verify(id.PublicKey, msg, sig)
}

// Returns the identitiy derived from the signed message. It will panic if
// len(data) is not long enough to contain data and signature.
func FromSignedData(data []byte) (*Identity, error) {
	signatureStart := len(data) - SIGNATURE_BYTE_LENGTH
	if signatureStart <= 0 {
		panic("identity: bad data length")
	}

	pubKey := data[signatureStart : signatureStart+ed25519.PublicKeySize]

	identity := NewPublicIdentity(pubKey)
	if !identity.VerifySignature(data) {
		return nil, errors.New("identity: invalid signature")
	}

	return identity, nil
}

func (id *Identity) Marshal() []byte {
	data := make([]byte, MARSHALLED_IDENTITY_TOTAL_SIZE)

	copy(data[MARSHALLED_IDENTITY_PUBLIC_KEY_START:MARSHALLED_IDENTITY_PUBLIC_KEY_END], id.PublicKey)
	copy(data[MARSHALLED_IDENTITY_PRIVATE_KEY_START:MARSHALLED_IDENTITY_PRIVATE_KEY_END], id.privateKey)

	return data
}

func Unmarshal(data []byte) (*Identity, error) {
	if len(data) != MARSHALLED_IDENTITY_TOTAL_SIZE {
		return nil, errors.New("identity: bad data length")
	}

	publicKey := data[MARSHALLED_IDENTITY_PUBLIC_KEY_START:MARSHALLED_IDENTITY_PUBLIC_KEY_END]
	privateKey := data[MARSHALLED_IDENTITY_PRIVATE_KEY_START:MARSHALLED_IDENTITY_PRIVATE_KEY_END]

	return newPrivateIdentity(publicKey, privateKey), nil
}

func newPrivateIdentity(publicKey []byte, privateKey []byte) *Identity {

	identity := NewPublicIdentity(publicKey)
	identity.privateKey = privateKey

	return identity
}
