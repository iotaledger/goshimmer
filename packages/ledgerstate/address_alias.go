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

// AliasAddress represents a special type of Address which is not backed by a private key directly,
// but is indirectly backed by a private key represented by AliasOutput
type AliasAddress struct {
	digest [32]byte
}

// NewAliasAddress creates a new AliasAddress from the given public key.
func NewAliasAddress(data []byte) *AliasAddress {
	return &AliasAddress{
		digest: blake2b.Sum256(data),
	}
}

// AliasAddressFromBytes unmarshals an AliasAddress from a sequence of bytes.
func AliasAddressFromBytes(bytes []byte) (address *AliasAddress, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if address, err = AliasAddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AliasAddress from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AliasAddressFromBase58EncodedString creates an AliasAddress from a base58 encoded string.
func AliasAddressFromBase58EncodedString(base58String string) (address *AliasAddress, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded AliasAddress (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if address, _, err = AliasAddressFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse AliasAddress from bytes: %w", err)
		return
	}

	return
}

// AliasAddressFromMarshalUtil parses a AliasAddress from the given MarshalUtil.
func AliasAddressFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (address *AliasAddress, err error) {
	addressType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("error parsing AddressType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if AddressType(addressType) != AliasAddressType {
		err = xerrors.Errorf("invalid AddressType (%X): %w", addressType, cerrors.ErrParseBytesFailed)
		return
	}

	data, err := marshalUtil.ReadBytes(32)
	if err != nil {
		err = xerrors.Errorf("error parsing digest (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	address = &AliasAddress{}
	copy(address.digest[:], data)
	return
}

func (b *AliasAddress) IsMint() bool {
	return b.digest == [32]byte{}
}

// Type returns the AddressType of the Address.
func (b *AliasAddress) Type() AddressType {
	return AliasAddressType
}

// Digest returns the hashed version of the Addresses public key.
func (b *AliasAddress) Digest() []byte {
	return b.digest[:]
}

// Clone creates a copy of the Address.
func (b *AliasAddress) Clone() Address {
	return &(*b)
}

// Bytes returns a marshaled version of the Address.
func (b *AliasAddress) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(AliasAddressType)}, b.digest[:])
}

// Array returns an array of bytes that contains the marshaled version of the Address.
func (b *AliasAddress) Array() (array [AddressLength]byte) {
	copy(array[:], b.Bytes())

	return
}

// Base58 returns a base58 encoded version of the Address.
func (b *AliasAddress) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the addresses for debug purposes.
func (b *AliasAddress) String() string {
	return stringify.Struct("AliasAddress",
		stringify.StructField("Digest", b.Digest()),
		stringify.StructField("Base58", b.Base58()),
	)
}

// code contract (make sure the struct implements all required methods)
var _ Address = &AliasAddress{}
