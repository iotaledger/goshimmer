package data

import (
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

func (dataPayload *Data) GetType() payload.Type {
	return dataPayload.payloadType
}

func (dataPayload *Data) GetData() []byte {
	return dataPayload.data
}

func (dataPayload *Data) UnmarshalBinary(data []byte) error {
	dataPayload.data = make([]byte, len(data))
	copy(dataPayload.data, data)

	return nil
}

func (dataPayload *Data) MarshalBinary() (data []byte, err error) {
	data = make([]byte, len(dataPayload.data))
	copy(data, dataPayload.data)

	return
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
