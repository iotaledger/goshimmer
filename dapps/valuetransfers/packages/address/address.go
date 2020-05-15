package address

import (
	"crypto/rand"
	"fmt"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/marshalutil"
)

// Version represents the version of the address. Different versions are associated to different signature schemes.
type Version = byte

// Digest represents a hashed version of the consumed public key of the address. Hashing the public key allows us to
// maintain quantum-robustness for addresses, that have never been spent from, and it allows us to have fixed size
// addresses.
type Digest = []byte

// Address represents an address in the IOTA ledger.
type Address [Length]byte

const (
	// VersionED25519 represents the address version that uses ED25519 signatures.
	VersionED25519 = byte(1)

	// VersionBLS represents the address version that uses BLS signatures.
	VersionBLS = byte(2)
)

// Random creates a random address, which can for example be used in unit tests.
// first byte (version) is also random
func Random() (address Address) {
	// generate a random sequence of bytes
	if _, err := rand.Read(address[:]); err != nil {
		panic(err)
	}
	return
}

// RandomOfType creates a random address with the given Version.
func RandomOfType(versionByte Version) Address {
	ret := Random()
	ret[0] = versionByte
	return ret
}

// FromBase58 creates an address from a base58 encoded string.
func FromBase58(base58String string) (address Address, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	// sanitize input
	if len(bytes) != Length {
		err = fmt.Errorf("base58 encoded string does not match the length of an address")

		return
	}

	// copy bytes to result
	copy(address[:], bytes)

	return
}

// FromED25519PubKey creates an address from an ed25519 public key.
func FromED25519PubKey(key ed25519.PublicKey) (address Address) {
	digest := blake2b.Sum256(key[:])

	address[0] = VersionED25519
	copy(address[1:], digest[:])

	return
}

// FromBLSPubKey creates an address from marshaled BLS public key
// unmarshaled BLS public key conforms to interface kyber.Point
func FromBLSPubKey(pubKey []byte) (address Address) {
	digest := blake2b.Sum256(pubKey)

	address[0] = VersionBLS
	copy(address[1:], digest[:])

	return
}

// FromBytes unmarshals an address from a sequence of bytes.
func FromBytes(bytes []byte) (result Address, consumedBytes int, err error) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	addressBytes, err := marshalUtil.ReadBytes(Length)
	if err != nil {
		return
	}
	copy(result[:], addressBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling of a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (Address, error) {
	address, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return FromBytes(data) })
	if err != nil {
		return Address{}, err
	}

	return address.(Address), nil
}

// Version returns the version of the address, which corresponds to the signature scheme that is used.
func (address *Address) Version() Version {
	return address[0]
}

// Digest returns the digest part of an address (i.e. the hashed version of the ed25519 public key)-
func (address *Address) Digest() Digest {
	return address[1:]
}

// Bytes returns a marshaled version of this address.
func (address Address) Bytes() []byte {
	return address[:]
}

// String returns a human readable (base58 encoded) version of the address.
func (address Address) String() string {
	return base58.Encode(address.Bytes())
}

// Length contains the length of an address (digest length = 32 + version byte length = 1).
const Length = 33
