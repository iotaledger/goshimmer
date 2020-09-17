package ledgerstate

import (
	"fmt"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/marshalutil"
)

// region AddressType //////////////////////////////////////////////////////////////////////////////////////////////////

// AddressType represents the type of the Address where different types encode different signature schemes.
type AddressType byte

// String returns a human readable representation of the AddressType.
func (a AddressType) String() string {
	return [...]string{
		"AddressTypeED25519",
		"AddressTypeBLS",
	}[a]
}

const (
	// AddressTypeED25519 represents an Address secured by the ED25519 signature scheme.
	AddressTypeED25519 AddressType = iota

	// AddressTypeBLS represents an Address secured by the BLS signature scheme.
	AddressTypeBLS
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Address //////////////////////////////////////////////////////////////////////////////////////////////////////

// Address is an interface for the different kind of Addresses that are support by the IOTA ledger.
type Address interface {
	// Type returns the AddressType of the Address.
	Type() AddressType

	// Digest returns the bytes of the Address that are usually comprised of the hashed version of the public key.
	Digest() []byte

	// Bytes returns a marshaled version of the Address.
	Bytes() []byte

	// Base58 returns a base58 encoded version of the address.
	Base58() string

	// String returns a human readable version of the addresses for debug purposes.
	String() string
}

// AddressFromBytes unmarshals an address from a sequence of bytes.
func AddressFromBytes(bytes []byte) (result Address, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = AddressFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AddressFromBase58EncodedString creates an Address from a base58 encoded string.
func AddressFromBase58EncodedString(base58String string) (address Address, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	address, _, err = AddressFromBytes(bytes)

	return
}

// AddressFromMarshalUtil reads an Address from the bytes in the given MarshalUtil.
func AddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result Address, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = fmt.Errorf("error while parsing OutputType: %w", err)

		return
	}
	marshalUtil.ReadSeek(-1)

	switch AddressType(outputType) {
	case AddressTypeED25519:
		return ED25519AddressFromMarshalUtil(marshalUtil)
	case AddressTypeBLS:
		return BLSAddressFromMarshalUtil(marshalUtil)
	default:
		err = fmt.Errorf("unable to parse address: unknown address type")
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ED25519Address ///////////////////////////////////////////////////////////////////////////////////////////////

// ED25519Address represents an Address that is secured by the ED25519 signature scheme.
type ED25519Address struct {
	digest []byte
}

// NewED25519Address creates a new ED25519Address from then given public key.
func NewED25519Address(publicKey ed25519.PublicKey) *ED25519Address {
	digest := blake2b.Sum256(publicKey[:])

	return &ED25519Address{
		digest: digest[:],
	}
}

// ED25519AddressFromMarshalUtil is a method that parses an ED25519Address from the given MarshalUtil.
func ED25519AddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address *ED25519Address, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = fmt.Errorf("error parsing OutputType in ED25519AddressFromMarshalUtil: %w", err)
		return
	}
	if AddressType(outputType) != AddressTypeBLS {
		err = fmt.Errorf("error parsing OutputType in BLSAddressFromMarshalUtil: invalid OutputType `%X`", outputType)
		return
	}

	address = &ED25519Address{}
	if address.digest, err = marshalUtil.ReadBytes(32); err != nil {
		err = fmt.Errorf("error parsing digest in ED25519AddressFromMarshalUtil: %w", err)
		return
	}

	return
}

// Type returns the AddressType of the Address.
func (e *ED25519Address) Type() AddressType {
	return AddressTypeED25519
}

// Digest returns the bytes of the Address that are usually comprised of the hashed version of the public key.
func (e *ED25519Address) Digest() []byte {
	return e.digest
}

// Bytes returns a marshaled version of the Address.
func (e *ED25519Address) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(AddressTypeED25519)}, e.digest)
}

// Base58 returns a base58 encoded version of the address.
func (e *ED25519Address) Base58() string {
	return base58.Encode(e.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (e *ED25519Address) String() string {
	return stringify.Struct("ED25519Address",
		stringify.StructField("Digest", e.Digest()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &ED25519Address{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BLSAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// BLSAddress represents an Address that is secured by the BLS signature scheme.
type BLSAddress struct {
	digest []byte
}

// NewBLSAddress creates a new BLSAddress from then given public key.
func NewBLSAddress(publicKey []byte) *BLSAddress {
	digest := blake2b.Sum256(publicKey)

	return &BLSAddress{
		digest: digest[:],
	}
}

// BLSAddressFromMarshalUtil parses a BLSAddress from the given MarshalUtil.
func BLSAddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address *BLSAddress, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = fmt.Errorf("error parsing OutputType in BLSAddressFromMarshalUtil: %w", err)
		return
	}
	if AddressType(outputType) != AddressTypeBLS {
		err = fmt.Errorf("error parsing OutputType in BLSAddressFromMarshalUtil: invalid OutputType `%X`", outputType)
		return
	}

	address = &BLSAddress{}
	if address.digest, err = marshalUtil.ReadBytes(32); err != nil {
		err = fmt.Errorf("error parsing digest in BLSAddressFromMarshalUtil: %w", err)
		return
	}

	return
}

// Type returns the AddressType of the Address.
func (b *BLSAddress) Type() AddressType {
	return AddressTypeBLS
}

// Digest returns the bytes of the Address that are usually comprised of the hashed version of the public key.
func (b *BLSAddress) Digest() []byte {
	return b.digest
}

// Bytes returns a marshaled version of the Address.
func (b *BLSAddress) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(AddressTypeBLS)}, b.digest)
}

// Base58 returns a base58 encoded version of the address.
func (b *BLSAddress) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (b *BLSAddress) String() string {
	return stringify.Struct("BLSAddress",
		stringify.StructField("Digest", b.Digest()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &BLSAddress{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
