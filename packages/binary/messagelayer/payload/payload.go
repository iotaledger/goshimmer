package payload

import (
	"github.com/iotaledger/hive.go/marshalutil"
)

func init() {
	// register the generic unmarshaler
	SetGenericUnmarshalerFactory(GenericPayloadUnmarshalerFactory)
	// register the generic data payload type
	RegisterType(DataType, GenericPayloadUnmarshalerFactory(DataType))
}

// Payload represents some kind of payload of data which only gains meaning by having
// corresponding node logic processing payloads of a given type.
type Payload interface {
	// Type returns the type of the payload.
	Type() Type
	// Bytes returns the payload bytes.
	Bytes() []byte
	// Unmarshal unmarshals the payload from the given bytes.
	Unmarshal(bytes []byte) error
	// String returns a human-friendly representation of the payload.
	String() string
}

// FromBytes unmarshals bytes into a payload.
func FromBytes(bytes []byte) (result Payload, consumedBytes int, err error) {
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

// Parse parses a payload by using the given marshal util.
func Parse(marshalUtil *marshalutil.MarshalUtil) (Payload, error) {
	if payload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return FromBytes(data) }); err != nil {
		return nil, err
	} else {
		return payload.(Payload), nil
	}
}
