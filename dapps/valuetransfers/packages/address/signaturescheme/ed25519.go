package signaturescheme

import (
	"fmt"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

// region PUBLIC API ///////////////////////////////////////////////////////////////////////////////////////////////////

// ED25519 creates an instance of a signature scheme, that is used to sign the corresponding address.
func ED25519(keyPair ed25519.KeyPair) SignatureScheme {
	return &ed25519SignatureScheme{
		keyPair: keyPair,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region signature scheme implementation //////////////////////////////////////////////////////////////////////////////

// ed25519SignatureScheme defines an interface for ED25519 elliptic curve signatures.
type ed25519SignatureScheme struct {
	keyPair ed25519.KeyPair
}

// Version returns the version byte that is associated to this signature scheme.
func (signatureScheme *ed25519SignatureScheme) Version() byte {
	return address.VERSION_ED25519
}

// Address returns the address that this signature scheme instance is securing.
func (signatureScheme *ed25519SignatureScheme) Address() address.Address {
	return address.FromED25519PubKey(signatureScheme.keyPair.PublicKey)
}

// Sign creates a valid signature for the given data according to the signature scheme implementation.
func (signatureScheme *ed25519SignatureScheme) Sign(data []byte) Signature {
	return &ed25519Signature{
		publicKey: signatureScheme.keyPair.PublicKey,
		signature: signatureScheme.keyPair.PrivateKey.Sign(data),
	}
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ SignatureScheme = &ed25519SignatureScheme{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region signature implementation /////////////////////////////////////////////////////////////////////////////////////

// ed25519Signature represents a signature for an addresses that uses elliptic curve cryptography.
type ed25519Signature struct {
	publicKey ed25519.PublicKey
	signature ed25519.Signature
}

// ed25519SignatureFromBytes unmarshals an ed25519 signatures from a sequence of bytes.
// It either creates a new signature or fills the optionally provided object with the parsed information.
func Ed25519SignatureFromBytes(bytes []byte, optionalTargetObject ...*ed25519Signature) (result *ed25519Signature, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &ed25519Signature{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ed25519SignatureFromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read version
	versionByte, err := marshalUtil.ReadByte()
	if err != nil {
		return
	} else if versionByte != address.VERSION_ED25519 {
		err = fmt.Errorf("invalid version byte when parsing ed25519 signature")

		return
	}

	// read public key
	publicKey, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return ed25519.PublicKeyFromBytes(data) })
	if err != nil {
		return
	}
	result.publicKey = publicKey.(ed25519.PublicKey)

	// read signature
	signature, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return ed25519.SignatureFromBytes(data) })
	if err != nil {
		return
	}
	result.signature = signature.(ed25519.Signature)

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// IsValid returns true if the signature is valid for the given data.
func (signature *ed25519Signature) IsValid(signedData []byte) bool {
	return signature.publicKey.VerifySignature(signedData, signature.signature)
}

// Bytes returns a marshaled version of the signature.
func (signature *ed25519Signature) Bytes() []byte {
	marshalUtil := marshalutil.New(1 + ed25519.PublicKeySize + ed25519.SignatureSize)
	marshalUtil.WriteByte(address.VERSION_ED25519)
	marshalUtil.WriteBytes(signature.publicKey[:])
	marshalUtil.WriteBytes(signature.signature[:])

	return marshalUtil.Bytes()
}

// Address returns the address, that this signature signs.
func (signature *ed25519Signature) Address() address.Address {
	return address.FromED25519PubKey(signature.publicKey)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
