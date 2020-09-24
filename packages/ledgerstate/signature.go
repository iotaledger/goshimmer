package ledgerstate

import (
	"bytes"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

// region SignatureType ////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// SignatureTypeED25519 represents an ED25519 signature.
	SignatureTypeED25519 SignatureType = iota

	// SignatureTypeBLS represents a BLS signature.
	SignatureTypeBLS
)

// SignatureType represents the type of the signature scheme.
type SignatureType uint8

// String returns a human readable representation of the SignatureType.
func (s SignatureType) String() string {
	return [...]string{
		"SignatureTypeED25519",
		"SignatureTypeBLS",
	}[s]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Signature ////////////////////////////////////////////////////////////////////////////////////////////////////

// Signature is an interface for the different kind of Signatures that are support by the ledger state.
type Signature interface {
	// Type returns the SignatureType of this Signature.
	Type() SignatureType

	// SignsData returns true if the Signature signs the given data.
	SignsData(data []byte) bool

	// SignsAddress returns true if the Signature signs the given address.
	SignsAddress(address Address, data []byte) bool

	// Bytes returns a marshaled version of the Signature.
	Bytes() []byte

	// String returns a human readable version of the Signature.
	String() string
}

// SignatureFromBytes unmarshals a Signature from a sequence of bytes.
func SignatureFromBytes(bytes []byte) (signature Signature, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if signature, err = SignatureFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Signature from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SignatureFromMarshalUtil unmarshals a Signature using a MarshalUtil (for easier unmarshaling).
func SignatureFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (signature Signature, err error) {
	signatureType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse SignatureType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch SignatureType(signatureType) {
	case SignatureTypeED25519:
		if signature, err = ED25519SignatureFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse ED25519Signature: %w", err)
			return
		}
	case SignatureTypeBLS:
		if signature, err = BLSSignatureFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse BLSSignature: %w", err)
			return
		}
	default:
		err = xerrors.Errorf("unsupported SignatureType (%X): %w", signatureType, ErrParseBytesFailed)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ED25519Signature /////////////////////////////////////////////////////////////////////////////////////////////

// ED25519Signature represents a Signature created with the ed25519 signature scheme.
type ED25519Signature struct {
	publicKey ed25519.PublicKey
	signature ed25519.Signature
}

// NewED25519Signature is the constructor of an ED25519Signature.
func NewED25519Signature(publicKey ed25519.PublicKey, signature ed25519.Signature) *ED25519Signature {
	return &ED25519Signature{
		publicKey: publicKey,
		signature: signature,
	}
}

// ED25519SignatureFromBytes unmarshals a ED25519Signature from a sequence of bytes.
func ED25519SignatureFromBytes(bytes []byte) (signature *ED25519Signature, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if signature, err = ED25519SignatureFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ED25519Signature from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ED25519SignatureFromMarshalUtil unmarshals a ED25519Signature using a MarshalUtil (for easier unmarshaling).
func ED25519SignatureFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (signature *ED25519Signature, err error) {
	signatureType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse SignatureType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if SignatureType(signatureType) != SignatureTypeED25519 {
		err = xerrors.Errorf("invalid SignatureType (%X): %w", signatureType, ErrParseBytesFailed)
		return
	}

	signature = &ED25519Signature{}
	if signature.publicKey, err = ed25519.ParsePublicKey(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse public key (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if signature.signature, err = ed25519.ParseSignature(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse signature (%v): %w", err, ErrParseBytesFailed)
		return
	}
	return
}

// Type returns the SignatureType of this Signature.
func (e *ED25519Signature) Type() SignatureType {
	return SignatureTypeED25519
}

// SignsData returns true if the Signature signs the given data.
func (e *ED25519Signature) SignsData(data []byte) bool {
	return e.publicKey.VerifySignature(data, e.signature)
}

// SignsAddress returns true if the Signature signs the given address.
func (e *ED25519Signature) SignsAddress(address Address, data []byte) bool {
	if address.Type() != AddressTypeED25519 {
		return false
	}

	hashedPublicKey := blake2b.Sum256(e.publicKey.Bytes())
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return e.SignsData(data)
}

// Bytes returns a marshaled version of the Signature.
func (e *ED25519Signature) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(SignatureTypeED25519)}, e.publicKey.Bytes(), e.signature.Bytes())
}

// String returns a human readable version of the Signature.
func (e *ED25519Signature) String() string {
	return stringify.Struct("ED25519Signature",
		stringify.StructField("publicKey", e.publicKey),
		stringify.StructField("signature", e.signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ Signature = &ED25519Signature{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BLSSignature /////////////////////////////////////////////////////////////////////////////////////////////////

type BLSSignature struct {
	publicKey []byte
	signature []byte
}

func BLSSignatureFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (signature *BLSSignature, err error) {
	return
}

// Type returns the SignatureType of this Signature.
func (b *BLSSignature) Type() SignatureType {
	return SignatureTypeBLS
}

// SignsData returns true if the Signature signs the given data.
func (b *BLSSignature) SignsData(data []byte) bool {
	panic("implement me")
}

// SignsAddress returns true if the Signature signs the given address.
func (b *BLSSignature) SignsAddress(address Address, data []byte) bool {
	if address.Type() != AddressTypeBLS {
		return false
	}

	hashedPublicKey := blake2b.Sum256(b.publicKey)
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return b.SignsData(data)
}

// Bytes returns a marshaled version of the Signature.
func (b *BLSSignature) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(SignatureTypeBLS)}, b.publicKey, b.signature)
}

// String returns a human readable version of the Signature.
func (b *BLSSignature) String() string {
	return stringify.Struct("BLSSignature",
		stringify.StructField("publicKey", b.publicKey),
		stringify.StructField("signature", b.signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ Signature = &BLSSignature{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
