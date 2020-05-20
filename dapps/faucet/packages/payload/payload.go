package faucetpayload

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

type Payload struct {
	payloadType payload.Type
	address     []byte
}

var Type = payload.Type(2)

func New(addr []byte) *Payload {
	return &Payload{
		payloadType: Type,
		address:     addr,
	}
}

func init() {
	payload.RegisterType(Type, GenericPayloadUnmarshalerFactory(Type))
}

func FromBytes(bytes []byte, optionalTargetObject ...*Payload) (result *Payload, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Payload{}
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
	result.address, err = marshalUtil.ReadBytes(int(payloadBytes))
	if err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (faucetPayload *Payload) Type() payload.Type {
	return faucetPayload.payloadType
}

func (faucetPayload *Payload) Address() []byte {
	return faucetPayload.address
}

// Bytes marshals the data payload into a sequence of bytes.
func (faucetPayload *Payload) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(faucetPayload.Type())
	marshalUtil.WriteUint32(uint32(len(faucetPayload.address)))
	marshalUtil.WriteBytes(faucetPayload.address[:])

	// return result
	return marshalUtil.Bytes()
}

func (faucetPayload *Payload) Unmarshal(data []byte) (err error) {
	_, err, _ = FromBytes(data, faucetPayload)

	return
}

func (faucetPayload *Payload) MarshalBinary() (data []byte, err error) {
	return faucetPayload.Bytes(), nil
}

func (faucetPayload *Payload) String() string {
	return stringify.Struct("FaucetPayload",
		stringify.StructField("address", string(faucetPayload.Address())),
	)
}

func GenericPayloadUnmarshalerFactory(payloadType payload.Type) payload.Unmarshaler {
	return func(data []byte) (payload payload.Payload, err error) {
		payload = &Payload{
			payloadType: payloadType,
		}
		err = payload.Unmarshal(data)

		return
	}
}
