package payload

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// DataType is the message type of a data payload.
var DataType = Type(0)

// Data represents a payload which just contains a blob of data.
type Data struct {
	payloadType Type
	data        []byte
}

// NewData creates new data payload.
func NewData(data []byte) *Data {
	return &Data{
		payloadType: DataType,
		data:        data,
	}
}

// DataFromBytes creates a new data payload from the given bytes.
func DataFromBytes(bytes []byte) (result *Data, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseData(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseData parses a new data payload out of the given marshal util.
func ParseData(marshalUtil *marshalutil.MarshalUtil) (result *Data, err error) {
	// parse information
	result = &Data{}
	payloadBytes, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	result.payloadType, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	result.data, err = marshalUtil.ReadBytes(int(payloadBytes))
	if err != nil {
		return
	}

	return
}

// Type returns the payload type.
func (dataPayload *Data) Type() Type {
	return dataPayload.payloadType
}

// Data returns the data of the data payload.
func (dataPayload *Data) Data() []byte {
	return dataPayload.data
}

// Bytes marshals the data payload into a sequence of bytes.
func (dataPayload *Data) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(uint32(len(dataPayload.data)))
	marshalUtil.WriteUint32(dataPayload.Type())
	marshalUtil.WriteBytes(dataPayload.data[:])

	// return result
	return marshalUtil.Bytes()
}

func (dataPayload *Data) String() string {
	return stringify.Struct("Data",
		stringify.StructField("type", int(dataPayload.Type())),
		stringify.StructField("data", string(dataPayload.Data())),
	)
}

// GenericPayloadUnmarshaler is an unmarshaler for the generic data payload type.
func GenericPayloadUnmarshaler(data []byte) (payload Payload, err error) {
	payload, _, err = DataFromBytes(data)

	return
}
