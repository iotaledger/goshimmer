package transaction

import (
	"fmt"

	"github.com/mr-tron/base58"
)

type Id [IdLength]byte

func NewId(base58EncodedString string) (result Id, err error) {
	bytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		return
	}

	if len(bytes) != IdLength {
		err = fmt.Errorf("length of base58 formatted transaction id is wrong")

		return
	}

	copy(result[:], bytes)

	return
}

func IdFromBytes(bytes []byte) (result Id, err error, consumedBytes int) {
	copy(result[:], bytes)

	consumedBytes = IdLength

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
