package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	// ObjectName defines the name of the data object.
	ObjectName = "data"
)

// DataType is the message type of a data payload.
var DataType = Type(0)

func init() {
	// register the generic unmarshaler
	SetGenericUnmarshalerFactory(GenericPayloadUnmarshalerFactory)
	// register the generic data payload type
	RegisterType(DataType, ObjectName, GenericPayloadUnmarshalerFactory(DataType))
}

// DataPayload represents a payload which just contains a blob of data.
type DataPayload struct {
	payloadType Type
	data        []byte
}

// NewDataPayload creates new data payload.
func NewDataPayload(data []byte) *DataPayload {
	return &DataPayload{
		payloadType: DataType,
		data:        data,
	}
}

// DataPayloadFromBytes creates a new data payload from the given bytes.
func DataPayloadFromBytes(bytes []byte, optionalTargetObject ...*DataPayload) (result *DataPayload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseDataPayload(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseDataPayload parses a new data payload out of the given marshal util.
func ParseDataPayload(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*DataPayload) (result *DataPayload, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &DataPayload{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ParseDataPayload")
	}

	// parse information
	result.payloadType, err = marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse data payload type: %w", err)
		return
	}
	payloadBytes, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse data payload size: %w", err)
		return
	}
	result.data, err = marshalUtil.ReadBytes(int(payloadBytes))
	if err != nil {
		err = fmt.Errorf("failed to parse data payload contest: %w", err)
		return
	}

	return
}

// Type returns the payload type.
func (d *DataPayload) Type() Type {
	return d.payloadType
}

// Data returns the data of the data payload.
func (d *DataPayload) Data() []byte {
	return d.data
}

// Bytes marshals the data payload into a sequence of bytes.
func (d *DataPayload) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(d.Type())
	marshalUtil.WriteUint32(uint32(len(d.data)))
	marshalUtil.WriteBytes(d.data[:])

	// return result
	return marshalUtil.Bytes()
}

// Unmarshal unmarshalls the byte array to a data payload.
func (d *DataPayload) Unmarshal(data []byte) (err error) {
	_, _, err = DataPayloadFromBytes(data, d)

	return
}

func (d *DataPayload) String() string {
	return stringify.Struct("Data",
		stringify.StructField("type", int(d.Type())),
		stringify.StructField("data", string(d.Data())),
	)
}

// GenericPayloadUnmarshalerFactory is an unmarshaler for the generic data payload type.
func GenericPayloadUnmarshalerFactory(payloadType Type) Unmarshaler {
	return func(data []byte) (payload Payload, err error) {
		payload = &DataPayload{payloadType: payloadType}
		err = payload.Unmarshal(data)
		return
	}
}
