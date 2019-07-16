package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/iotaledger/goshimmer/packages/crypto"
)

type Identity struct {
	Type             IdentityType
	Identifier       []byte
	StringIdentifier string
	PublicKey        []byte
	PrivateKey       []byte
}

func NewIdentity(publicKey []byte, optionalPrivateKey ...[]byte) *Identity {
	this := &Identity{
		Identifier: crypto.Hash20(publicKey),
		PublicKey:  make([]byte, len(publicKey)),
	}

	copy(this.PublicKey, publicKey)

	this.StringIdentifier = fmt.Sprintf("%x", this.Identifier)

	if len(optionalPrivateKey) == 1 {
		this.Type = PRIVATE_TYPE
		this.PrivateKey = optionalPrivateKey[0]
	} else {
		this.Type = PUBLIC_TYPE
	}

	return this
}

func (this *Identity) Sign(data []byte) ([]byte, error) {
	sha256Hasher := sha256.New()
	sha256Hasher.Write(data)

	sig, err := secp256k1.Sign(sha256Hasher.Sum(nil), this.PrivateKey)
	if err != nil {
		return nil, err
	}

	return sig, nil
}

func (this *Identity) VerifySignature(data []byte, signature []byte) bool {
	sha256Hasher := sha256.New()
	sha256Hasher.Write(data)

	return secp256k1.VerifySignature(this.PublicKey, sha256Hasher.Sum(nil), signature[:64])
}

func GenerateRandomIdentity() *Identity {
	// generate key pair
	keyPair, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	// build public key bytes
	publicKey := elliptic.Marshal(secp256k1.S256(), keyPair.X, keyPair.Y)

	// build private key bytes
	privkey := make([]byte, 32)
	blob := keyPair.D.Bytes()
	copy(privkey[32-len(blob):], blob)

	return NewIdentity(publicKey, privkey)
}

func FromSignedData(data []byte, signature []byte) (*Identity, error) {
	sha256Hasher := sha256.New()
	sha256Hasher.Write(data)

	pubKey, err := secp256k1.RecoverPubkey(sha256Hasher.Sum(nil), signature)
	if err != nil {
		return nil, err
	}

	return NewIdentity(pubKey), nil
}
