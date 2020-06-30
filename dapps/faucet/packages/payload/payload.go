package faucetpayload

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

const (
	// ObjectName defines the name of the facuet object.
	ObjectName = "faucet"
)

// Payload represents a request which contains an address for the faucet to send funds to.
type Payload struct {
	payloadType payload.Type
	address     address.Address
}

// Type represents the identifier for the faucet Payload type.
var Type = payload.Type(2)

// New is the constructor of a Payload and creates a new Payload object from the given details.
func New(addr address.Address) *Payload {
	return &Payload{
		payloadType: Type,
		address:     addr,
	}
}

func init() {
	payload.RegisterType(Type, ObjectName, GenericPayloadUnmarshalerFactory(Type))
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
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
	addr, err := marshalUtil.ReadBytes(int(payloadBytes))
	if err != nil {
		return
	}
	result.address, _, _ = address.FromBytes(addr)

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Type returns the type of the faucet Payload.
func (faucetPayload *Payload) Type() payload.Type {
	return faucetPayload.payloadType
}

// Address returns the address of the faucet Payload.
func (faucetPayload *Payload) Address() address.Address {
	return faucetPayload.address
}

// Bytes marshals the data payload into a sequence of bytes.
func (faucetPayload *Payload) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(faucetPayload.Type())
	marshalUtil.WriteUint32(uint32(len(faucetPayload.address)))
	marshalUtil.WriteBytes(faucetPayload.address.Bytes())

	// return result
	return marshalUtil.Bytes()
}

// Unmarshal unmarshals a given slice of bytes and fills the object.
func (faucetPayload *Payload) Unmarshal(data []byte) (err error) {
	_, err, _ = FromBytes(data, faucetPayload)

	return
}

// String returns a human readable version of faucet payload (for debug purposes).
func (faucetPayload *Payload) String() string {
	return stringify.Struct("FaucetPayload",
		stringify.StructField("address", faucetPayload.Address().String()),
	)
}

// GenericPayloadUnmarshalerFactory sets the generic unmarshaler.
func GenericPayloadUnmarshalerFactory(payloadType payload.Type) payload.Unmarshaler {
	return func(data []byte) (payload payload.Payload, err error) {
		payload = &Payload{
			payloadType: payloadType,
		}
		err = payload.Unmarshal(data)

		return
	}
}

// IsFaucetReq checks if the message is faucet payload.
func IsFaucetReq(msg *message.Message) bool {
	return msg.Payload().Type() == Type
}
