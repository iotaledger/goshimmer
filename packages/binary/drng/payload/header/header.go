package header

import (
	"github.com/iotaledger/hive.go/marshalutil"
)

type Type = byte

type payloadType struct {
	CollectiveBeacon Type
}

var drngTypes = &payloadType{
	CollectiveBeacon: Type(1),
}

const Length = 5

func CollectiveBeaconType() Type {
	return drngTypes.CollectiveBeacon
}

type Header struct {
	payloadType Type   // message type
	instanceID  uint32 // identifier of the dRAND instance
}

func New(payloadType Type, instanceID uint32) Header {
	return Header{
		payloadType: payloadType,
		instanceID:  instanceID,
	}
}

func (h Header) PayloadType() Type {
	return h.payloadType
}

func (h Header) Instance() uint32 {
	return h.instanceID
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (Header, error) {
	if header, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return Header{}, err
	} else {
		return header.(Header), nil
	}
}

// FromBytes unmarshals a header from a sequence of bytes.
// It either creates a new header or fills the optionally provided object with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Header) (result Header, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	var targetObject *Header
	switch len(optionalTargetObject) {
	case 0:
		targetObject = &result
	case 1:
		targetObject = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read payload type from bytes
	if targetObject.payloadType, err = marshalUtil.ReadByte(); err != nil {
		return
	}

	// read instance ID from bytes
	if targetObject.instanceID, err = marshalUtil.ReadUint32(); err != nil {
		return
	}

	// copy result if we have provided a target object
	result = *targetObject

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (header *Header) Bytes() (bytes []byte) {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteByte(header.PayloadType())
	marshalUtil.WriteUint32(header.Instance())

	bytes = marshalUtil.Bytes()

	return
}
