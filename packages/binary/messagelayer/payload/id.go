package payload

import "github.com/mr-tron/base58"

// ID represents the id of a data payload.
type ID [IDLength]byte

// Bytes returns the id as a byte slice backed by the original array,
// therefore it should not be modified.
func (id ID) Bytes() []byte {
	return id[:]
}

func (id ID) String() string {
	return base58.Encode(id[:])
}

// IDLength is the length of a data payload id.
const IDLength = 64
