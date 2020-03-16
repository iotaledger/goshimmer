package identity

import (
	"crypto/rand"

	"github.com/oasislabs/ed25519"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
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

// ParsePublicIdentity is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParsePublicIdentity(marshalUtil *marshalutil.MarshalUtil) (*Identity, error) {
	if identity, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return PublicIdentityFromBytes(data) }); err != nil {
		return nil, err
	} else {
		return identity.(*Identity), nil
	}
}

// PublicIdentityFromBytes unmarshals a public identity from a sequence of bytes.
func PublicIdentityFromBytes(bytes []byte, optionalTargetObject ...*Identity) (result *Identity, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Identity{
			Type: Public,
		}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to PublicIdentityFromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read public key from bytes
	publicKeyBytes, err := marshalUtil.ReadBytes(PublicKeySize)
	if err != nil {
		return
	}
	result.PublicKey = make([]byte, PublicKeySize)
	copy(result.PublicKey, publicKeyBytes)

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
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
