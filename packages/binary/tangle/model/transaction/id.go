package transaction

import (
	"github.com/mr-tron/base58"
)

type Id [IdLength]byte

func NewId(id []byte) (result Id) {
	copy(result[:], id)

	return
}

func (id *Id) MarshalBinary() (result []byte, err error) {
	result = make([]byte, IdLength)
	copy(result, id[:])

	return
}

func (id *Id) UnmarshalBinary(data []byte) (err error) {
	copy(id[:], data)

	return
}

func (id Id) String() string {
	return base58.Encode(id[:])
}

var EmptyId = Id{}

const IdLength = 64
