package faucetpayload

import (
	"context"
	"crypto"
	"fmt"

	// Only want to use init
	_ "golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

const (
	// ObjectName defines the name of the facuet object.
	ObjectName = "faucet"
)

// Payload represents a request which contains an address for the faucet to send funds to.
type Payload struct {
	payloadType tangle.PayloadType
	address     address.Address
	nonce       uint64
}

// Type represents the identifier for the faucet Payload type.
var Type = tangle.PayloadType(2)
var powWorker = pow.New(crypto.BLAKE2b_512, 1)

// New is the constructor of a Payload and creates a new Payload object from the given details.
func New(addr address.Address, powTarget int) (*Payload, error) {
	p := &Payload{
		payloadType: Type,
		address:     addr,
	}

	payloadBytes := p.Bytes()
	powRelevantBytes := payloadBytes[:len(payloadBytes)-pow.NonceBytes]
	nonce, err := powWorker.Mine(context.Background(), powRelevantBytes, powTarget)
	if err != nil {
		err = fmt.Errorf("failed to do PoW for faucet payload: %w", err)
		return nil, err
	}
	p.nonce = nonce
	return p, nil
}

func init() {
	tangle.RegisterPayloadType(Type, ObjectName, PayloadUnmarshaler)
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read data
	result = &Payload{}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to unmarshal payload size of faucet payload from bytes: %w", err)
		return
	}
	result.payloadType, err = marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to unmarshal payload type of faucet payload from bytes: %w", err)
		return
	}
	addr, err := marshalUtil.ReadBytes(address.Length)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal address of faucet payload from bytes: %w", err)
		return
	}
	result.address, _, err = address.FromBytes(addr)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal address of faucet payload from bytes: %w", err)
		return
	}

	result.nonce, err = marshalUtil.ReadUint64()
	if err != nil {
		err = fmt.Errorf("failed to unmarshal nonce of faucet payload from bytes: %w", err)
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Type returns the type of the faucet Payload.
func (p *Payload) Type() tangle.PayloadType {
	return p.payloadType
}

// Address returns the address of the faucet Payload.
func (p *Payload) Address() address.Address {
	return p.address
}

// Bytes marshals the data payload into a sequence of bytes.
func (p *Payload) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(uint32(address.Length + pow.NonceBytes))
	marshalUtil.WriteUint32(p.Type())
	marshalUtil.WriteBytes(p.address.Bytes())
	marshalUtil.WriteUint64(p.nonce)

	// return result
	return marshalUtil.Bytes()
}

// String returns a human readable version of faucet payload (for debug purposes).
func (p *Payload) String() string {
	return stringify.Struct("FaucetPayload",
		stringify.StructField("address", p.Address().String()),
	)
}

// PayloadUnmarshaler sets the generic unmarshaler.
func PayloadUnmarshaler(data []byte) (payload tangle.Payload, err error) {
	payload, _, err = FromBytes(data)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal faucet payload from bytes: %w", err)
	}

	return
}

// IsFaucetReq checks if the message is faucet payload.
func IsFaucetReq(msg *tangle.Message) bool {
	return msg.Payload().Type() == Type
}
