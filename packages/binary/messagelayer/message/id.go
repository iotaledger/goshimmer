package message

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// ContentID identifies the content of a message without its trunk/branch ids.
type ContentID = ID

// ID identifies a message in its entirety. Unlike the sole content id, it also incorporates
// the trunk and branch ids.
type ID [IDLength]byte

// NewID creates a new message id.
func NewID(base58EncodedString string) (result ID, err error) {
	bytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		return
	}

	if len(bytes) != IDLength {
		err = fmt.Errorf("length of base58 formatted message id is wrong")

		return
	}

	copy(result[:], bytes)

	return
}

// IDFromBytes unmarshals a message id from a sequence of bytes.
func IDFromBytes(bytes []byte) (result ID, consumedBytes int, err error) {
	// check arguments
	if len(bytes) < IDLength {
		err = fmt.Errorf("bytes not long enough to encode a valid message id")
	}

	// calculate result
	copy(result[:], bytes)

	// return the number of bytes we processed
	consumedBytes = IDLength

	return
}

// ParseID is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParseID(marshalUtil *marshalutil.MarshalUtil) (ID, error) {
	id, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return IDFromBytes(data) })
	if err != nil {
		return ID{}, err
	}
	return id.(ID), nil
}

// MarshalBinary marshals the ID into bytes.
func (id *ID) MarshalBinary() (result []byte, err error) {
	return id.Bytes(), nil
}

// UnmarshalBinary unmarshals the bytes into an ID.
func (id *ID) UnmarshalBinary(data []byte) (err error) {
	copy(id[:], data)

	return
}

// Bytes returns the bytes of the ID.
func (id ID) Bytes() []byte {
	return id[:]
}

// String returns the base58 encode of the ID.
func (id ID) String() string {
	return base58.Encode(id[:])
}

// EmptyID is an empty id.
var EmptyID = ID{}

// IDLength defines the length of an ID.
const IDLength = 64
