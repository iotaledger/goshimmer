package message

import (
	"fmt"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
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

// IdFromBytes unmarshals a transaction id from a sequence of bytes.
func IdFromBytes(bytes []byte) (result Id, err error, consumedBytes int) {
	// check arguments
	if len(bytes) < IdLength {
		err = fmt.Errorf("bytes not long enough to encode a valid transaction id")
	}

	// calculate result
	copy(result[:], bytes)

	// return the number of bytes we processed
	consumedBytes = IdLength

	return
}

// ParseId is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParseId(marshalUtil *marshalutil.MarshalUtil) (Id, error) {
	if id, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return IdFromBytes(data) }); err != nil {
		return Id{}, err
	} else {
		return id.(Id), nil
	}
}

func (id *Id) MarshalBinary() (result []byte, err error) {
	return id.Bytes(), nil
}

func (id *Id) UnmarshalBinary(data []byte) (err error) {
	copy(id[:], data)

	return
}

func (id Id) Bytes() []byte {
	return id[:]
}

func (id Id) String() string {
	return base58.Encode(id[:])
}

var EmptyId = Id{}

const IdLength = 64
