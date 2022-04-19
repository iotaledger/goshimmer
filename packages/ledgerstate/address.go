package ledgerstate

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/serix"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

//nolint:dupl
func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(new(ED25519Address), serix.TypeSettings{}.WithObjectCode(new(ED25519Address).Type()))
	if err != nil {
		panic(fmt.Errorf("error registering ED25519Address type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(new(BLSAddress), serix.TypeSettings{}.WithObjectCode(new(BLSAddress).Type()))
	if err != nil {
		panic(fmt.Errorf("error registering BLSAddress type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(new(AliasAddress), serix.TypeSettings{}.WithObjectCode(new(AliasAddress).Type()))
	if err != nil {
		panic(fmt.Errorf("error registering AliasAddress type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*Address)(nil), new(ED25519Address), new(BLSAddress), new(AliasAddress))
	if err != nil {
		panic(fmt.Errorf("error registering Address interface implementations: %w", err))
	}
}

// region AddressType //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// ED25519AddressType represents an Address secured by the ED25519 signature scheme.
	ED25519AddressType AddressType = iota

	// BLSAddressType represents an Address secured by the BLS signature scheme.
	BLSAddressType

	// AliasAddressType represents ID used in AliasOutput and AliasLockOutput.
	AliasAddressType
)

// AddressLength contains the length of an address (type length = 1, Digest2 length = 32).
const AddressLength = 33

// AddressType represents the type of the Address (different types encode different signature schemes).
type AddressType byte

// String returns a human readable representation of the AddressType.
func (a AddressType) String() string {
	return [...]string{
		"AddressTypeED25519",
		"AddressTypeBLS",
		"AliasAddress",
	}[a]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Address //////////////////////////////////////////////////////////////////////////////////////////////////////

// Address is an interface for the different kind of Addresses that are supported by the ledger state.
type Address interface {
	// Type returns the AddressType of the Address.
	Type() AddressType

	// Digest returns the hashed version of the Addresses public key.
	Digest() []byte

	// Clone creates a copy of the Address.
	Clone() Address

	// Equals returns true if the two Addresses are equal.
	Equals(other Address) bool

	// Bytes returns a marshaled version of the Address.
	Bytes() []byte

	// Array returns an array of bytes that contains the marshaled version of the Address.
	Array() [AddressLength]byte

	// Base58 returns a base58 encoded version of the Address.
	Base58() string

	// String returns a human readable version of the Address for debug purposes.
	String() string
}

// AddressFromBytes unmarshals an Address from a sequence of bytes.
func AddressFromBytes(bytes []byte) (address Address, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Address from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AddressFromBase58EncodedString creates an Address from a base58 encoded string.
func AddressFromBase58EncodedString(base58String string) (address Address, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded Address (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if address, _, err = AddressFromBytes(bytes); err != nil {
		err = errors.Errorf("failed to parse Address from bytes: %w", err)
		return
	}

	return
}

// AddressFromMarshalUtil reads an Address from the bytes in the given MarshalUtil.
func AddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address Address, err error) {
	addressType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse AddressType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch AddressType(addressType) {
	case ED25519AddressType:
		return ED25519AddressFromMarshalUtil(marshalUtil)
	case BLSAddressType:
		return BLSAddressFromMarshalUtil(marshalUtil)
	case AliasAddressType:
		return AliasAddressFromMarshalUtil(marshalUtil)
	default:
		err = errors.Errorf("unsupported address type (%X): %w", addressType, cerrors.ErrParseBytesFailed)
		return
	}
}

// AddressFromSignature returns address corresponding to the signature if it has one (for ed25519 and BLS).
func AddressFromSignature(sig Signature) (Address, error) {
	switch s := sig.(type) {
	case *ED25519Signature:
		return NewED25519Address(s.PublicKey), nil
	case *BLSSignature:
		return NewBLSAddress(s.Signature.PublicKey.Bytes()), nil
	}
	return nil, errors.New("signature has no corresponding address")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ED25519Address ///////////////////////////////////////////////////////////////////////////////////////////////

// ED25519Address represents an Address that is secured by the ED25519 signature scheme.
type ED25519Address struct {
	ed25519AddressInner `serix:"0"`
}
type ed25519AddressInner struct {
	Digest2 [blake2b.Size256]byte `serix:"0"`
}

// NewED25519Address creates a new ED25519Address from the given public key.
func NewED25519Address(publicKey ed25519.PublicKey) *ED25519Address {
	return &ED25519Address{
		ed25519AddressInner: ed25519AddressInner{
			Digest2: blake2b.Sum256(publicKey[:]),
		},
	}
}

// ED25519AddressFromBytes unmarshals an ED25519Address from a sequence of bytes.
func ED25519AddressFromBytes(bytes []byte) (address *ED25519Address, consumedBytes int, err error) {
	// TODO: replace with FromBytesNew eventually

	marshalUtil := marshalutil.New(bytes)
	if address, err = ED25519AddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ED25519Address from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BLSAddressFromBytes unmarshals an BLSAddress from a sequence of bytes.
func ED25519AddressFromBytesNew(bytes []byte) (address *ED25519Address, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, address, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// ED25519AddressFromBase58EncodedString creates an ED25519Address from a base58 encoded string.
func ED25519AddressFromBase58EncodedString(base58String string) (address *ED25519Address, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded ED25519Address (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if address, _, err = ED25519AddressFromBytes(bytes); err != nil {
		err = errors.Errorf("failed to parse ED25519Address from bytes: %w", err)
		return
	}

	return
}

// ED25519AddressFromMarshalUtil is a method that parses an ED25519Address from the given MarshalUtil.
func ED25519AddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address *ED25519Address, err error) {
	addressType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse AddressType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if AddressType(addressType) != ED25519AddressType {
		err = errors.Errorf("invalid AddressType (%X): %w", addressType, cerrors.ErrParseBytesFailed)
		return
	}

	address = &ED25519Address{}
	data, err := marshalUtil.ReadBytes(32)
	if err != nil {
		err = errors.Errorf("error parsing Digest2 (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(address.Digest2[:], data)

	return
}

// Type returns the AddressType of the Address.
func (e *ED25519Address) Type() AddressType {
	return ED25519AddressType
}

// Digest returns the hashed version of the Addresses public key.
func (e *ED25519Address) Digest() []byte {
	return e.Digest2[:]
}

// Clone creates a copy of the Address.
func (e *ED25519Address) Clone() Address {
	return &ED25519Address{
		ed25519AddressInner{Digest2: e.Digest2},
	}
}

// Equals returns true if the two Addresses are equal.
func (e *ED25519Address) Equals(other Address) bool {
	return e.Type() == other.Type() && bytes.Equal(e.Digest(), other.Digest())
}

// Bytes returns a marshaled version of the Address.
func (e *ED25519Address) BytesOld() []byte {
	// TODO: remove eventually
	return byteutils.ConcatBytes([]byte{byte(ED25519AddressType)}, e.Digest())
}

// Bytes returns a marshaled version of the Address.
func (e *ED25519Address) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), e, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (e *ED25519Address) Array() (array [AddressLength]byte) {
	copy(array[:], e.Bytes())

	return
}

// Base58 returns a base58 encoded version of the address.
func (e *ED25519Address) Base58() string {
	return base58.Encode(e.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (e *ED25519Address) String() string {
	return stringify.Struct("ED25519Address",
		stringify.StructField("Digest", e.Digest()),
		stringify.StructField("Base58", e.Base58()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &ED25519Address{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BLSAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// BLSAddress represents an Address that is secured by the BLS signature scheme.
type BLSAddress struct {
	blsAddressInner `serix:"0"`
}
type blsAddressInner struct {
	Digest [blake2b.Size256]byte `serix:"0"`
}

// NewBLSAddress creates a new BLSAddress from the given public key.
func NewBLSAddress(publicKey []byte) *BLSAddress {
	digest := blake2b.Sum256(publicKey)

	return &BLSAddress{
		blsAddressInner{
			Digest: digest,
		},
	}
}

// BLSAddressFromBytes unmarshals an BLSAddress from a sequence of bytes.
func BLSAddressFromBytesNew(bytes []byte) (address *BLSAddress, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, address, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// BLSAddressFromBytes unmarshals an BLSAddress from a sequence of bytes.
func BLSAddressFromBytes(bytes []byte) (address *BLSAddress, consumedBytes int, err error) {
	// TODO: replace with FromBytesNew eventually
	marshalUtil := marshalutil.New(bytes)
	if address, err = BLSAddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BLSAddress from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BLSAddressFromBase58EncodedString creates an BLSAddress from a base58 encoded string.
func BLSAddressFromBase58EncodedString(base58String string) (address *BLSAddress, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded BLSAddress (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if address, _, err = BLSAddressFromBytes(bytes); err != nil {
		err = errors.Errorf("failed to parse BLSAddress from bytes: %w", err)
		return
	}

	return
}

// BLSAddressFromMarshalUtil parses a BLSAddress from the given MarshalUtil.
func BLSAddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address *BLSAddress, err error) {
	addressType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("error parsing AddressType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if AddressType(addressType) != BLSAddressType {
		err = errors.Errorf("invalid AddressType (%X): %w", addressType, cerrors.ErrParseBytesFailed)
		return
	}

	address = &BLSAddress{}
	data, err := marshalUtil.ReadBytes(32)
	if err != nil {
		err = errors.Errorf("error parsing Digest2 (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(address.blsAddressInner.Digest[:], data)

	return
}

// Type returns the AddressType of the Address.
func (b *BLSAddress) Type() AddressType {
	return BLSAddressType
}

// Digest returns the hashed version of the Addresses public key.
func (b *BLSAddress) Digest() []byte {
	return b.blsAddressInner.Digest[:]
}

// Clone creates a copy of the Address.
func (b *BLSAddress) Clone() Address {
	return &BLSAddress{
		blsAddressInner{
			Digest: b.blsAddressInner.Digest,
		},
	}
}

// Equals returns true if the two Addresses are equal.
func (b *BLSAddress) Equals(other Address) bool {
	return b.Type() == other.Type() && bytes.Equal(b.Digest(), other.Digest())
}

// Bytes returns a marshaled version of the Address.
func (b *BLSAddress) BytesOld() []byte {
	// TODO: remove eventually
	return byteutils.ConcatBytes([]byte{byte(BLSAddressType)}, b.blsAddressInner.Digest[:])
}

// Bytes returns a marshaled version of the Address.
func (b *BLSAddress) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (b *BLSAddress) Array() (array [AddressLength]byte) {
	copy(array[:], b.Bytes())

	return
}

// Base58 returns a base58 encoded version of the Address.
func (b *BLSAddress) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (b *BLSAddress) String() string {
	return stringify.Struct("BLSAddress",
		stringify.StructField("Digest", b.Digest()),
		stringify.StructField("Base58", b.Base58()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &BLSAddress{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// AliasAddressDigestSize defines the length of the alias address Digest2 in bytes.
const AliasAddressDigestSize = 32

// AliasAddress represents a special type of Address which is not backed by a private key directly,
// but is indirectly backed by a private key defined by corresponding AliasOutput parameters.
type AliasAddress struct {
	aliasAddressInner `serix:"0"`
}
type aliasAddressInner struct {
	Digest [AliasAddressDigestSize]byte `serix:"0"`
}

// NewAliasAddress creates a new AliasAddress from the given bytes used as seed.
// Normally the seed is an OutputID.
func NewAliasAddress(data []byte) *AliasAddress {
	return &AliasAddress{
		aliasAddressInner{
			Digest: blake2b.Sum256(data),
		},
	}
}

// AliasAddressFromBytes unmarshals an AliasAddress from a sequence of bytes.
func AliasAddressFromBytesNew(data []byte) (address *AliasAddress, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, address, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// AliasAddressFromBytes unmarshals an AliasAddress from a sequence of bytes.
func AliasAddressFromBytes(data []byte) (address *AliasAddress, consumedBytes int, err error) {
	// TODO: replace with FromBytesNew eventually
	marshalUtil := marshalutil.New(data)
	if address, err = AliasAddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse AliasAddress from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AliasAddressFromBase58EncodedString creates an AliasAddress from a base58 encoded string.
func AliasAddressFromBase58EncodedString(base58String string) (address *AliasAddress, err error) {
	data, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded AliasAddress (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if address, _, err = AliasAddressFromBytes(data); err != nil {
		err = errors.Errorf("failed to parse AliasAddress from data: %w", err)
		return
	}

	return
}

// AliasAddressFromMarshalUtil parses a AliasAddress from the given MarshalUtil.
func AliasAddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address *AliasAddress, err error) {
	addressType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("error parsing AddressType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if AddressType(addressType) != AliasAddressType {
		err = errors.Errorf("invalid AddressType (%X): %w", addressType, cerrors.ErrParseBytesFailed)
		return
	}

	data, err := marshalUtil.ReadBytes(AliasAddressDigestSize)
	if err != nil {
		err = errors.Errorf("error parsing Digest2 (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	address = &AliasAddress{}
	copy(address.aliasAddressInner.Digest[:], data)
	return
}

// Type returns the AddressType of the Address.
func (a *AliasAddress) Type() AddressType {
	return AliasAddressType
}

// Digest returns the hashed version of the Addresses public key.
func (a *AliasAddress) Digest() []byte {
	return a.aliasAddressInner.Digest[:]
}

// Clone creates a copy of the Address.
func (a *AliasAddress) Clone() Address {
	return &AliasAddress{aliasAddressInner{Digest: a.aliasAddressInner.Digest}}
}

// Bytes returns a marshaled version of the Address.
func (a *AliasAddress) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Bytes returns a marshaled version of the Address.
func (a *AliasAddress) BytesOld() []byte {
	// TODO: remove eventually
	return byteutils.ConcatBytes([]byte{byte(AliasAddressType)}, a.aliasAddressInner.Digest[:])
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (a *AliasAddress) Array() (array [AddressLength]byte) {
	copy(array[:], a.Bytes())

	return
}

// Equals returns true if the two Addresses are equal.
func (a *AliasAddress) Equals(other Address) bool {
	return a.Type() == other.Type() && bytes.Equal(a.Digest(), other.Digest())
}

// Base58 returns a base58 encoded version of the Address.
func (a *AliasAddress) Base58() string {
	return base58.Encode(a.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (a *AliasAddress) String() string {
	return stringify.Struct("AliasAddress",
		stringify.StructField("Digest", a.Digest()),
		stringify.StructField("Base58", a.Base58()),
	)
}

// IsNil returns if the alias address is zero value (uninitialized).
func (a *AliasAddress) IsNil() bool {
	return a.aliasAddressInner.Digest == [32]byte{}
}

// code contract (make sure the struct implements all required methods).
var _ Address = &AliasAddress{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////