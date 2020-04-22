package payload

import "github.com/mr-tron/base58"

// Id represents the id of a data payload.
type Id [IdLength]byte

// Bytes returns the id as a byte slice backed by the original array,
// therefore it should not be modified.
func (id Id) Bytes() []byte {
	return id[:]
}

func (id Id) String() string {
	return base58.Encode(id[:])
}

// IdLength is the length of a data payload id.
const IdLength = 64
