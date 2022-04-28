package ledgerstate

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(BLSSignature{}, serix.TypeSettings{}.WithObjectType(uint8(new(BLSSignature).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering BLSSignature type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(ED25519Signature{}, serix.TypeSettings{}.WithObjectType(uint8(new(ED25519Signature).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering ED25519Signature type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*Signature)(nil), new(BLSSignature), new(ED25519Signature))
	if err != nil {
		panic(fmt.Errorf("error registering Signature interface implementations: %w", err))
	}
}

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
func SignatureFromBytes(data []byte) (signature Signature, consumedBytes int, err error) {
	var signatureType SignatureType
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &signatureType)
	if err != nil {
		err = errors.Errorf("failed to parse SignatureType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	switch signatureType {
	case ED25519SignatureType:
		if signature, consumedBytes, err = ED25519SignatureFromBytes(data); err != nil {
			err = errors.Errorf("failed to parse ED25519Signature: %w", err)
			return
		}
	case BLSSignatureType:
		if signature, consumedBytes, err = BLSSignatureFromBytes(data); err != nil {
			err = errors.Errorf("failed to parse BLSSignature: %w", err)
			return
		}
	default:
		err = errors.Errorf("unsupported SignatureType (%X): %w", signatureType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// SignatureFromBase58EncodedString creates a Signature from a base58 encoded string.
func SignatureFromBase58EncodedString(base58String string) (signature Signature, err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded Signature (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if signature, _, err = SignatureFromBytes(decodedBytes); err != nil {
		err = errors.Errorf("failed to parse Signature from bytes: %w", err)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ED25519Signature /////////////////////////////////////////////////////////////////////////////////////////////

// ED25519Signature represents a Signature created with the ed25519 signature scheme.
type ED25519Signature struct {
	PublicKey ed25519.PublicKey `serix:"0"`
	Signature ed25519.Signature `serix:"1"`
}

// NewED25519Signature is the constructor of an ED25519Signature.
func NewED25519Signature(publicKey ed25519.PublicKey, signature ed25519.Signature) *ED25519Signature {
	return &ED25519Signature{
		PublicKey: publicKey,
		Signature: signature,
	}
}

// ED25519SignatureFromBytes unmarshals a ED25519Signature from a sequence of bytes.
func ED25519SignatureFromBytes(bytes []byte) (signature *ED25519Signature, consumedBytes int, err error) {
	signature = new(ED25519Signature)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, signature, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// Type returns the SignatureType of this Signature.
func (e *ED25519Signature) Type() SignatureType {
	return ED25519SignatureType
}

// SignatureValid returns true if the Signature signs the given data.
func (e *ED25519Signature) SignatureValid(data []byte) bool {
	return e.PublicKey.VerifySignature(data, e.Signature)
}

// AddressSignatureValid returns true if the Signature signs the given Address.
func (e *ED25519Signature) AddressSignatureValid(address Address, data []byte) bool {
	if address.Type() != ED25519AddressType {
		return false
	}

	hashedPublicKey := blake2b.Sum256(e.PublicKey.Bytes())
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return e.SignatureValid(data)
}

// Bytes returns a marshaled version of the Signature.
func (e *ED25519Signature) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), e, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Base58 returns a base58 encoded version of the Signature.
func (e *ED25519Signature) Base58() string {
	return base58.Encode(e.Bytes())
}

// String returns a human readable version of the Signature.
func (e *ED25519Signature) String() string {
	return stringify.Struct("ED25519Signature",
		stringify.StructField("publicKey", e.PublicKey),
		stringify.StructField("signature", e.Signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ Signature = &ED25519Signature{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BLSSignature /////////////////////////////////////////////////////////////////////////////////////////////////

// BLSSignature represents a Signature created with the BLS signature scheme.
type BLSSignature struct {
	Signature bls.SignatureWithPublicKey `serix:"0"`
}

// NewBLSSignature is the constructor of a BLSSignature.
func NewBLSSignature(signature bls.SignatureWithPublicKey) *BLSSignature {
	return &BLSSignature{
		Signature: signature,
	}
}

// BLSSignatureFromBytes unmarshals a BLSSignature from a sequence of bytes.
func BLSSignatureFromBytes(bytes []byte) (signature *BLSSignature, consumedBytes int, err error) {
	signature = new(BLSSignature)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, signature, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// Type returns the SignatureType of this Signature.
func (b *BLSSignature) Type() SignatureType {
	return BLSSignatureType
}

// SignatureValid returns true if the Signature signs the given data.
func (b *BLSSignature) SignatureValid(data []byte) bool {
	return b.Signature.IsValid(data)
}

// AddressSignatureValid returns true if the Signature signs the given Address.
func (b *BLSSignature) AddressSignatureValid(address Address, data []byte) bool {
	if address.Type() != BLSAddressType {
		return false
	}

	hashedPublicKey := blake2b.Sum256(b.Signature.PublicKey.Bytes())
	if !bytes.Equal(hashedPublicKey[:], address.Digest()) {
		return false
	}

	return b.SignatureValid(data)
}

// Bytes returns a marshaled version of the Signature.
func (b *BLSSignature) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Base58 returns a base58 encoded version of the Signature.
func (b *BLSSignature) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the Signature.
func (b *BLSSignature) String() string {
	return stringify.Struct("BLSSignature",
		stringify.StructField("publicKey", b.Signature.PublicKey),
		stringify.StructField("signature", b.Signature.Signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ Signature = &BLSSignature{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
