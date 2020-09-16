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
var DataType = PayloadType(0)

func init() {
	// register the generic data payload type
	RegisterPayloadType(DataType, ObjectName, GenericPayloadUnmarshaler)
}

// DataPayload represents a payload which just contains a blob of data.
type DataPayload struct {
	payloadType PayloadType
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
func DataPayloadFromBytes(bytes []byte) (result *DataPayload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = DataPayloadFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// DataPayloadFromMarshalUtil parses a new data payload out of the given marshal util.
func DataPayloadFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *DataPayload, err error) {
	// parse information
	result = &DataPayload{}
	payloadBytes, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse data payload size: %w", err)
		return
	}
	result.payloadType, err = marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse data payload type: %w", err)
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
func (d *DataPayload) Type() PayloadType {
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
	marshalUtil.WriteUint32(uint32(len(d.data)))
	marshalUtil.WriteUint32(d.Type())
	marshalUtil.WriteBytes(d.data[:])

	// return result
	return marshalUtil.Bytes()
}

func (d *DataPayload) String() string {
	return stringify.Struct("Data",
		stringify.StructField("type", int(d.Type())),
		stringify.StructField("data", string(d.Data())),
	)
}

// GenericPayloadUnmarshaler is an unmarshaler for the generic data payload type.
func GenericPayloadUnmarshaler(data []byte) (payload Payload, err error) {
	payload, _, err = DataPayloadFromBytes(data)

	return
}
