package drng

import (
	"github.com/iotaledger/hive.go/marshalutil"
)

// Type defines a drng payload type
type Type = byte

const (
	// TypeCollectiveBeacon defines a CollectiveBeacon payload type
	TypeCollectiveBeacon Type = 1
)

// HeaderLength defines the length of a DRNG header
const HeaderLength = 5

// Header defines defines a DRNG payload header
type Header struct {
	PayloadType Type   // message type
	InstanceID  uint32 // identifier of the DRNG instance
}

// NewHeader creates a new DRNG payload header for the given type and instance id.
func NewHeader(payloadType Type, instanceID uint32) Header {
	return Header{
		PayloadType: payloadType,
		InstanceID:  instanceID,
	}
}

// HeaderFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func HeaderFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (Header, error) {
	header, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return HeaderFromBytes(data) })
	if err != nil {
		return Header{}, err
	}
	return header.(Header), nil
}

// HeaderFromBytes unmarshals a header from a sequence of bytes.
// It either creates a new header or fills the optionally provided object with the parsed information.
func HeaderFromBytes(bytes []byte, optionalTargetObject ...*Header) (result Header, consumedBytes int, err error) {
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
	if targetObject.PayloadType, err = marshalUtil.ReadByte(); err != nil {
		return
	}

	// read instance ID from bytes
	if targetObject.InstanceID, err = marshalUtil.ReadUint32(); err != nil {
		return
	}

	// copy result if we have provided a target object
	result = *targetObject

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Bytes returns the header in serialized bytes form.
func (h *Header) Bytes() (bytes []byte) {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteByte(h.PayloadType)
	marshalUtil.WriteUint32(h.InstanceID)

	bytes = marshalUtil.Bytes()

	return
}
