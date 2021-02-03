package ledgerstate

import (
	"bytes"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

// region SignatureType ////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// ED25519SignatureType represents an ED25519 Signature.
	ED25519SignatureType SignatureType = iota

	// BLSSignatureType represents a BLS Signature.
	BLSSignatureType
)

// SignatureType represents the type of the signature scheme.
type SignatureType uint8

// String returns a human readable representation of the SignatureType.
func (s SignatureType) String() string {
	return [...]string{
		"ED25519SignatureType",
		"BLSSignatureType",
	}[s]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Signature ////////////////////////////////////////////////////////////////////////////////////////////////////

// Signature is an interface for the different kinds of Signatures that are supported by the ledger state.
type Signature interface {
	// Type returns the SignatureType of this Signature.
	Type() SignatureType

	// SignatureValid returns true if the Signature signs the given data.
	SignatureValid(data []byte) bool

	// AddressSignatureValid returns true if the Signature signs the given Address.
	AddressSignatureValid(address Address, data []byte) bool

	// Bytes returns a marshaled version of the Signature.
	Bytes() []byte

	// Base58 returns a base58 encoded version of the Signature.
	Base58() string

	// String returns a human readable version of the Signature.
	String() string
}

// SignatureFromBytes unmarshals a Signature from a sequence of bytes.
func SignatureFromBytes(bytes []byte) (signature Signature, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if signature, err = SignatureFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Signature from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SignatureFromBase58EncodedString creates a Signature from a base58 encoded string.
func SignatureFromBase58EncodedString(base58String string) (signature Signature, err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded Signature (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if signature, _, err = SignatureFromBytes(decodedBytes); err != nil {
		err = xerrors.Errorf("failed to parse Signature from bytes: %w", err)
		return
	}

	return
}

// SignatureFromMarshalUtil unmarshals a Signature using a MarshalUtil (for easier unmarshaling).
func SignatureFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (signature Signature, err error) {
	signatureType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse SignatureType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch SignatureType(signatureType) {
	case ED25519SignatureType:
		if signature, err = ED25519SignatureFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse ED25519Signature: %w", err)
			return
		}
	case BLSSignatureType:
		if signature, err = BLSSignatureFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse BLSSignature: %w", err)
			return
		}
	default:
		err = xerrors.Errorf("unsupported SignatureType (%X): %w", signatureType, cerrors.ErrParseBytesFailed)
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
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ED25519SignatureFromBase58EncodedString creates an ED25519Signature from a base58 encoded string.
func ED25519SignatureFromBase58EncodedString(base58String string) (signature *ED25519Signature, err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded ED25519Signature (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if signature, _, err = ED25519SignatureFromBytes(decodedBytes); err != nil {
		err = xerrors.Errorf("failed to parse ED25519Signature from bytes: %w", err)
		return
	}

	return
}

// ED25519SignatureFromMarshalUtil unmarshals a ED25519Signature using a MarshalUtil (for easier unmarshaling).
func ED25519SignatureFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (signature *ED25519Signature, err error) {
	signatureType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse SignatureType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if SignatureType(signatureType) != ED25519SignatureType {
		err = xerrors.Errorf("invalid SignatureType (%X): %w", signatureType, cerrors.ErrParseBytesFailed)
		return
	}

	signature = &ED25519Signature{}
	if signature.publicKey, err = ed25519.ParsePublicKey(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse public key (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if signature.signature, err = ed25519.ParseSignature(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse signature (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// Type returns the SignatureType of this Signature.
func (e *ED25519Signature) Type() SignatureType {
	return ED25519SignatureType
}

// SignatureValid returns true if the Signature signs the given data.
func (e *ED25519Signature) SignatureValid(data []byte) bool {
	return e.publicKey.VerifySignature(data, e.signature)
}

// AddressSignatureValid returns true if the Signature signs the given Address.
func (e *ED25519Signature) AddressSignatureValid(address Address, data []byte) bool {
	if address.Type() != ED25519AddressType {
		return false
	}

	hashedPublicKey := blake2b.Sum256(e.publicKey.Bytes())
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return e.SignatureValid(data)
}

// Bytes returns a marshaled version of the Signature.
func (e *ED25519Signature) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(ED25519SignatureType)}, e.publicKey.Bytes(), e.signature.Bytes())
}

// Base58 returns a base58 encoded version of the Signature.
func (e *ED25519Signature) Base58() string {
	return base58.Encode(e.Bytes())
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

// BLSSignature represents a Signature created with the BLS signature scheme.
type BLSSignature struct {
	signature bls.SignatureWithPublicKey
}

// NewBLSSignature is the constructor of a BLSSignature.
func NewBLSSignature(signature bls.SignatureWithPublicKey) *BLSSignature {
	return &BLSSignature{
		signature: signature,
	}
}

// BLSSignatureFromBytes unmarshals a BLSSignature from a sequence of bytes.
func BLSSignatureFromBytes(bytes []byte) (signature *BLSSignature, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if signature, err = BLSSignatureFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BLSSignature from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BLSSignatureFromBase58EncodedString creates a BLSSignature from a base58 encoded string.
func BLSSignatureFromBase58EncodedString(base58String string) (signature *BLSSignature, err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded BLSSignature (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if signature, _, err = BLSSignatureFromBytes(decodedBytes); err != nil {
		err = xerrors.Errorf("failed to parse BLSSignature from bytes: %w", err)
		return
	}

	return
}

// BLSSignatureFromMarshalUtil unmarshals a BLSSignature using a MarshalUtil (for easier unmarshaling).
func BLSSignatureFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (signature *BLSSignature, err error) {
	signatureType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse SignatureType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if SignatureType(signatureType) != BLSSignatureType {
		err = xerrors.Errorf("invalid SignatureType (%X): %w", signatureType, cerrors.ErrParseBytesFailed)
		return
	}

	signature = &BLSSignature{}
	if signature.signature, err = bls.SignatureWithPublicKeyFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SignatureWithPublicKey from MarshalUtil (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Type returns the SignatureType of this Signature.
func (b *BLSSignature) Type() SignatureType {
	return BLSSignatureType
}

// SignatureValid returns true if the Signature signs the given data.
func (b *BLSSignature) SignatureValid(data []byte) bool {
	return b.signature.IsValid(data)
}

// AddressSignatureValid returns true if the Signature signs the given Address.
func (b *BLSSignature) AddressSignatureValid(address Address, data []byte) bool {
	if address.Type() != BLSAddressType {
		return false
	}

	hashedPublicKey := blake2b.Sum256(b.signature.PublicKey.Bytes())
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return b.SignatureValid(data)
}

// Bytes returns a marshaled version of the Signature.
func (b *BLSSignature) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(BLSSignatureType)}, b.signature.Bytes())
}

// Base58 returns a base58 encoded version of the Signature.
func (b *BLSSignature) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the Signature.
func (b *BLSSignature) String() string {
	return stringify.Struct("BLSSignature",
		stringify.StructField("publicKey", b.signature.PublicKey),
		stringify.StructField("signature", b.signature.Signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ Signature = &BLSSignature{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
