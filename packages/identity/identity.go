package identity

import (
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

// Verifies whether the data contains a valid signature of the message.
func (id *Identity) VerifySignature(data []byte) error {
	signatureStart := len(data) - SIGNATURE_BYTE_LENGTH
	if signatureStart <= 0 {
		return ErrInvalidDataLen
	}

	msg := data[:signatureStart]

	// ignore the public key
	sig := data[signatureStart+ed25519.PublicKeySize:]

	if !ed25519.Verify(id.PublicKey, msg, sig) {
		return ErrInvalidSignature
	}

	return nil
}

// Returns the identitiy derived from the signed message.
func FromSignedData(data []byte) (*Identity, error) {
	signatureStart := len(data) - SIGNATURE_BYTE_LENGTH
	if signatureStart <= 0 {
		return nil, ErrInvalidDataLen
	}

	pubKey := data[signatureStart : signatureStart+ed25519.PublicKeySize]

	identity := NewPublicIdentity(pubKey)
	if err := identity.VerifySignature(data); err != nil {
		return nil, err
	}

	return identity, nil
}

func (id *Identity) Marshal() []byte {
	data := make([]byte, MARSHALED_IDENTITY_TOTAL_SIZE)

	copy(data[MARSHALED_IDENTITY_PUBLIC_KEY_START:MARSHALED_IDENTITY_PUBLIC_KEY_END], id.PublicKey)
	copy(data[MARSHALED_IDENTITY_PRIVATE_KEY_START:MARSHALED_IDENTITY_PRIVATE_KEY_END], id.privateKey)

	return data
}

func Unmarshal(data []byte) (*Identity, error) {
	if len(data) != MARSHALED_IDENTITY_TOTAL_SIZE {
		return nil, ErrInvalidDataLen
	}

	publicKey := data[MARSHALED_IDENTITY_PUBLIC_KEY_START:MARSHALED_IDENTITY_PUBLIC_KEY_END]
	privateKey := data[MARSHALED_IDENTITY_PRIVATE_KEY_START:MARSHALED_IDENTITY_PRIVATE_KEY_END]

	return newPrivateIdentity(publicKey, privateKey), nil
}

func newPrivateIdentity(publicKey []byte, privateKey []byte) *Identity {

	identity := NewPublicIdentity(publicKey)
	identity.privateKey = privateKey

	return identity
}
