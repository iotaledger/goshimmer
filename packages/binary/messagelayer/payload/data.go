package payload

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

var DataType = Type(0)

type Data struct {
	payloadType Type
	data        []byte
}

func NewData(data []byte) *Data {
	return &Data{
		payloadType: DataType,
		data:        data,
	}
}

func DataFromBytes(bytes []byte, optionalTargetObject ...*Data) (result *Data, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseData(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseData(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Data) (result *Data, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Data{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ParseData")
	}

	// parse information
	result.payloadType, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	payloadBytes, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	result.data, err = marshalUtil.ReadBytes(int(payloadBytes))
	if err != nil {
		return
	}

	return
}

func (dataPayload *Data) Type() Type {
	return dataPayload.payloadType
}

func (dataPayload *Data) Data() []byte {
	return dataPayload.data
}

// Bytes marshals the data payload into a sequence of bytes.
func (dataPayload *Data) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(dataPayload.Type())
	marshalUtil.WriteUint32(uint32(len(dataPayload.data)))
	marshalUtil.WriteBytes(dataPayload.data[:])

	// return result
	return marshalUtil.Bytes()
}

func (dataPayload *Data) Unmarshal(data []byte) (err error) {
	_, err, _ = DataFromBytes(data, dataPayload)

	return
}

func (dataPayload *Data) String() string {
	return stringify.Struct("Data",
		stringify.StructField("data", string(dataPayload.Data())),
	)
}

func GenericPayloadUnmarshalerFactory(payloadType Type) Unmarshaler {
	return func(data []byte) (payload Payload, err error) {
		payload = &Data{
			payloadType: payloadType,
		}
		err = payload.Unmarshal(data)

		return
	}
}
