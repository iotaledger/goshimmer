package transaction

import (
	"crypto/rand"
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// ID is the data type that represents the identifier for a Transaction.
type ID [IDLength]byte

// IDFromBase58 creates an id from a base58 encoded string.
func IDFromBase58(base58String string) (id ID, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	// sanitize input
	if len(bytes) != IDLength {
		err = fmt.Errorf("base58 encoded string does not match the length of a transaction id")

		return
	}

	// copy bytes to result
	copy(id[:], bytes)

	return
}

// IDFromBytes unmarshals an ID from a sequence of bytes.
func IDFromBytes(bytes []byte) (result ID, consumedBytes int, err error) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	idBytes, idErr := marshalUtil.ReadBytes(IDLength)
	if idErr != nil {
		err = idErr

		return
	}
	copy(result[:], idBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseID is a wrapper for simplified unmarshaling of Ids from a byte stream using the marshalUtil package.
func ParseID(marshalUtil *marshalutil.MarshalUtil) (ID, error) {
	id, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return IDFromBytes(data) })
	if err != nil {
		return ID{}, err
	}

	return id.(ID), nil
}

// RandomID creates a random id which can for example be used in unit tests.
func RandomID() (id ID) {
	// generate a random sequence of bytes
	idBytes := make([]byte, IDLength)
	if _, err := rand.Read(idBytes); err != nil {
		panic(err)
	}

	// copy the generated bytes into the result
	copy(id[:], idBytes)

	return
}

// Bytes marshals the ID into a sequence of bytes.
func (id ID) Bytes() []byte {
	return id[:]
}

// String creates a human readable version of the ID (for debug purposes).
func (id ID) String() string {
	return base58.Encode(id[:])
}

// GenesisID represents the genesis ID.
var GenesisID ID

// IDLength contains the amount of bytes that a marshaled version of the ID contains.
const IDLength = 32
