package faucet

import (
	"context"
	"crypto"
	"fmt"

	// Only want to use init
	_ "golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	// ObjectName defines the name of the facuet object (payload).
	ObjectName = "faucet"
)

// region Object implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type represents the identifier for the faucet Object type.
var Type = tangle.PayloadType(2)

// Object represents a faucet request which contains an address for the faucet to send funds to.
type Object struct {
	payloadType tangle.PayloadType
	address     address.Address
	nonce       uint64
}

// Type returns the type of the faucet Object.
func (o *Object) Type() tangle.PayloadType {
	return o.payloadType
}

// Address returns the address of the faucet Object.
func (o *Object) Address() address.Address {
	return o.address
}

// Bytes marshals the faucet object into a sequence of bytes.
func (o *Object) Bytes() (bytes []byte) {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(uint32(address.Length + pow.NonceBytes))
	marshalUtil.WriteUint32(o.Type())
	marshalUtil.WriteBytes(o.address.Bytes())
	marshalUtil.WriteUint64(o.nonce)

	// return result
	return marshalUtil.Bytes()
}

// String returns a human readable version of faucet payload (for debug purposes).
func (o *Object) String() string {
	return stringify.Struct("FaucetPayload",
		stringify.StructField("address", o.Address().String()),
	)
}

// ObjectUnmarshaler sets the generic unmarshaler.
func ObjectUnmarshaler(data []byte) (payload tangle.Payload, err error) {
	payload, _, err = FromBytes(data)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal faucet payload from bytes: %w", err)
	}

	return
}

func init() {
	tangle.RegisterPayloadType(Type, ObjectName, ObjectUnmarshaler)
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

var powWorker = pow.New(crypto.BLAKE2b_512, 1)

// NewObject is the constructor of an Object and creates a new Object object from the given details.
func NewObject(addr address.Address, powTarget int) (*Object, error) {
	p := &Object{
		payloadType: Type,
		address:     addr,
	}

	objectBytes := p.Bytes()
	powRelevantBytes := objectBytes[:len(objectBytes)-pow.NonceBytes]
	nonce, err := powWorker.Mine(context.Background(), powRelevantBytes, powTarget)
	if err != nil {
		err = fmt.Errorf("failed to do PoW for faucet payload: %w", err)
		return nil, err
	}
	p.nonce = nonce
	return p, nil
}

// FromBytes parses the marshaled version of a Object into an object.
// It either returns a new Object or fills an optionally provided Object with the parsed information.
func FromBytes(bytes []byte) (result *Object, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	result = &Object{}
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
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// IsFaucetReq checks if the message is faucet payload.
func IsFaucetReq(msg *tangle.Message) bool {
	return msg.Payload().Type() == Type
}
