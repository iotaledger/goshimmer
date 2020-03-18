package id

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

type Id [Length]byte

func New(idBytes []byte) (result Id) {
	copy(result[:], idBytes)

	return
}

// FromBytes unmarshals a transfer id from a sequence of bytes.
func FromBytes(bytes []byte) (result Id, err error, consumedBytes int) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	idBytes, err := marshalUtil.ReadBytes(Length)
	if err != nil {
		return
	}
	copy(result[:], idBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (Id, error) {
	if id, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return Id{}, err
	} else {
		return id.(Id), nil
	}
}

func (id Id) Bytes() []byte {
	return id[:]
}

func (id Id) String() string {
	return base58.Encode(id[:])
}

const Length = 32
