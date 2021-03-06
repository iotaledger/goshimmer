package ledgerstate

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

// AddressAlias represents an Address that is secured by the AliasLocks
type AddressAlias struct {
	digest []byte
}

// NewAddressAlias creates a new AddressAlias from the given public key.
func NewAddressAlias(data []byte) *AddressAlias {
	digest := blake2b.Sum256(data)

	return &AddressAlias{
		digest: digest[:],
	}
}

// AddressAliasFromBytes unmarshals an AddressAlias from a sequence of bytes.
func AddressAliasFromBytes(bytes []byte) (address *AddressAlias, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if address, err = AddressAliasFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AddressAlias from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AddressAliasFromBase58EncodedString creates an AddressAlias from a base58 encoded string.
func AddressAliasFromBase58EncodedString(base58String string) (address *AddressAlias, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded AddressAlias (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if address, _, err = AddressAliasFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse AddressAlias from bytes: %w", err)
		return
	}

	return
}

// AddressAliasFromMarshalUtil parses a AddressAlias from the given MarshalUtil.
func AddressAliasFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address *AddressAlias, err error) {
	addressType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("error parsing AddressType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if AddressType(addressType) != AddressAliasType {
		err = xerrors.Errorf("invalid AddressType (%X): %w", addressType, cerrors.ErrParseBytesFailed)
		return
	}

	address = &AddressAlias{}
	if address.digest, err = marshalUtil.ReadBytes(32); err != nil {
		err = xerrors.Errorf("error parsing digest (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Type returns the AddressType of the Address.
func (b *AddressAlias) Type() AddressType {
	return AddressAliasType
}

// Digest returns the hashed version of the Addresses public key.
func (b *AddressAlias) Digest() []byte {
	return b.digest
}

// Clone creates a copy of the Address.
func (b *AddressAlias) Clone() Address {
	clonedDigest := make([]byte, len(b.digest))
	copy(clonedDigest, b.digest)

	return &AddressAlias{
		digest: clonedDigest,
	}
}

// Bytes returns a marshaled version of the Address.
func (b *AddressAlias) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(AddressAliasType)}, b.digest)
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (b *AddressAlias) Array() (array [AddressLength]byte) {
	copy(array[:], b.Bytes())

	return
}

// Base58 returns a base58 encoded version of the Address.
func (b *AddressAlias) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (b *AddressAlias) String() string {
	return stringify.Struct("AddressAlias",
		stringify.StructField("Digest", b.Digest()),
		stringify.StructField("Base58", b.Base58()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &AddressAlias{}
