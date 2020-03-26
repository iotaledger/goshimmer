package payload

import (
	"github.com/iotaledger/hive.go/marshalutil"
)

type Payload interface {
	Type() Type
	Bytes() []byte
	Unmarshal(bytes []byte) error
	String() string
}

// FromBytes unmarshals a public identity from a sequence of bytes.
func FromBytes(bytes []byte) (result Payload, err error, consumedBytes int) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// calculate result
	payloadType, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	marshalUtil.ReadSeek(marshalUtil.ReadOffset() - marshalutil.UINT32_SIZE*2)
	payloadBytes, err := marshalUtil.ReadBytes(int(payloadSize) + 8)
	if err != nil {
		return
	}
	result, err = GetUnmarshaler(payloadType)(payloadBytes)
	if err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (Payload, error) {
	if payload, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return nil, err
	} else {
		return payload.(Payload), nil
	}
}
