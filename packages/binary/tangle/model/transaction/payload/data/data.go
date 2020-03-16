package data

import (
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
)

type Data struct {
	payloadType payload.Type
	data        []byte
}

var Type = payload.Type(0)

func New(data []byte) *Data {
	return &Data{
		payloadType: Type,
		data:        data,
	}
}

func FromBytes(bytes []byte, optionalTargetObject ...*Data) (result *Data, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Data{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read data
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

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (dataPayload *Data) GetType() payload.Type {
	return dataPayload.payloadType
}

func (dataPayload *Data) GetData() []byte {
	return dataPayload.data
}

// Bytes marshals the data payload into a sequence of bytes.
func (dataPayload *Data) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(dataPayload.GetType())
	marshalUtil.WriteUint32(uint32(len(dataPayload.data)))
	marshalUtil.WriteBytes(dataPayload.data[:])

	// return result
	return marshalUtil.Bytes()
}

func (dataPayload *Data) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = FromBytes(data, dataPayload)

	return
}

func (dataPayload *Data) MarshalBinary() (data []byte, err error) {
	return dataPayload.Bytes(), nil
}

func (dataPayload *Data) String() string {
	return stringify.Struct("Data",
		stringify.StructField("data", string(dataPayload.GetData())),
	)
}

func GenericPayloadUnmarshalerFactory(payloadType payload.Type) payload.Unmarshaler {
	return func(data []byte) (payload payload.Payload, err error) {
		payload = &Data{
			payloadType: payloadType,
		}
		err = payload.UnmarshalBinary(data)

		return
	}
}
