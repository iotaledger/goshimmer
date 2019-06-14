package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/iotaledger/goshimmer/packages/crypto"
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
	publicKey, privateKey, err := generateKey()
	if err != nil {
		panic(err)
	}

	return newPrivateIdentity(publicKey, privateKey)
}

// Sign signs the message with privateKey and returns the message plus the signature.
func (id *Identity) AddSignature(msg []byte) []byte {
	signatureStart := len(msg)

	hash := sha256.Sum256(msg)
	signature, err := secp256k1.Sign(hash[:], id.privateKey)
	if err != nil {
		panic(err)
	}

	data := make([]byte, signatureStart+SIGNATURE_BYTE_LENGTH)

	copy(data[:signatureStart], msg)
	copy(data[signatureStart:], signature)

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
	hash := sha256.Sum256(msg)

	sig := data[signatureStart:]
	return secp256k1.VerifySignature(id.PublicKey, hash[:], sig)
}

// Returns the identitiy derived from the signed message. It will panic if
// len(data) is not long enough to contain data and signature.
func FromSignedData(data []byte) (*Identity, error) {
	signatureStart := len(data) - SIGNATURE_BYTE_LENGTH
	if signatureStart <= 0 {
		panic("identity: bad data length")
	}

	msg := data[:signatureStart]
	hash := sha256.Sum256(msg)

	sig := data[signatureStart:]
	pubKey, err := secp256k1.RecoverPubkey(hash[:], sig)
	if err != nil {
		return nil, err
	}

	identity := NewPublicIdentity(pubKey)
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

func generateKey() ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	pubkey := elliptic.Marshal(secp256k1.S256(), key.X, key.Y)

	privkey := make([]byte, 32)
	blob := key.D.Bytes()
	copy(privkey[32-len(blob):], blob)

	return pubkey, privkey, nil
}
