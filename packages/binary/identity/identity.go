package identity

import (
	"crypto/rand"

	"github.com/oasislabs/ed25519"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
)

type Identity struct {
	Type       Type
	PublicKey  ed25119.PublicKey
	PrivateKey ed25119.PrivateKey
}

func New(publicKey []byte, optionalPrivateKey ...[]byte) (result *Identity, err error) {
	result = &Identity{}

	result.PublicKey, err, _ = ed25119.PublicKeyFromBytes(publicKey)
	if err != nil {
		return
	}

	if len(optionalPrivateKey) == 0 {
		result.Type = Public
	} else {
		result.Type = Private
		result.PrivateKey, err, _ = ed25119.PrivateKeyFromBytes(optionalPrivateKey[0])
		if err != nil {
			return
		}
	}

	return
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
	result.PublicKey, err = ed25119.ParsePublicKey(marshalUtil)
	if err != nil {
		return
	}

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

func (identity *Identity) Sign(data []byte) ed25119.Signature {
	return identity.PrivateKey.Sign(data)
}

func (identity *Identity) VerifySignature(data []byte, signature []byte) bool {
	return ed25519.Verify(identity.PublicKey, data, signature)
}
