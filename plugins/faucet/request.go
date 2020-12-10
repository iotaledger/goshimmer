package faucet

import (
	"context"
	"crypto"

	"github.com/iotaledger/hive.go/cerrors"
	"golang.org/x/xerrors"

	// Only want to use init
	_ "golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	// ObjectName defines the name of the faucet object (payload).
	ObjectName = "faucet"
)

// Request represents a faucet request which contains an address for the faucet to send funds to.
type Request struct {
	payloadType payload.Type
	address     address.Address
	nonce       uint64
}

// Type represents the identifier for the faucet Request type.
var Type = payload.NewType(2, ObjectName, PayloadUnmarshaler)
var powWorker = pow.New(crypto.BLAKE2b_512, 1)

// NewRequest is the constructor of a Request and creates a new Request object from the given details.
func NewRequest(addr address.Address, powTarget int) (*Request, error) {
	p := &Request{
		payloadType: Type,
		address:     addr,
	}

	objectBytes := p.Bytes()
	powRelevantBytes := objectBytes[:len(objectBytes)-pow.NonceBytes]
	nonce, err := powWorker.Mine(context.Background(), powRelevantBytes, powTarget)
	if err != nil {
		err = xerrors.Errorf("failed to do PoW for faucet request: %w", err)
		return nil, err
	}
	p.nonce = nonce
	return p, nil
}

// FromBytes parses the marshaled version of a Request into a request object.
func FromBytes(bytes []byte) (result *Request, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	result = &Request{}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse payload size (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.payloadType, err = payload.TypeFromMarshalUtil(marshalUtil)
	if err != nil {
		err = xerrors.Errorf("failed to parse Type from MarshalUtil: %w", err)
		return
	}
	addr, err := marshalUtil.ReadBytes(address.Length)
	if err != nil {
		err = xerrors.Errorf("failed to parse address of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.address, _, err = address.FromBytes(addr)
	if err != nil {
		err = xerrors.Errorf("failed to unmarshal address of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	result.nonce, err = marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to unmarshal nonce of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Type returns the type of the faucet Request.
func (p *Request) Type() payload.Type {
	return p.payloadType
}

// Address returns the address of the faucet Request.
func (p *Request) Address() address.Address {
	return p.address
}

// Bytes marshals the faucet Request payload into a sequence of bytes.
func (p *Request) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(payload.TypeLength + uint32(address.Length+pow.NonceBytes))
	marshalUtil.WriteBytes(p.Type().Bytes())
	marshalUtil.WriteBytes(p.address.Bytes())
	marshalUtil.WriteUint64(p.nonce)

	// return result
	return marshalUtil.Bytes()
}

// String returns a human readable version of faucet Request payload (for debug purposes).
func (p *Request) String() string {
	return stringify.Struct("FaucetPayload",
		stringify.StructField("address", p.Address().String()),
	)
}

// PayloadUnmarshaler sets the generic unmarshaler.
func PayloadUnmarshaler(data []byte) (payload payload.Payload, err error) {
	payload, _, err = FromBytes(data)
	if err != nil {
		err = xerrors.Errorf("failed to unmarshal faucet payload from bytes: %w", err)
	}

	return
}

// IsFaucetReq checks if the message is faucet payload.
func IsFaucetReq(msg *tangle.Message) bool {
	return msg.Payload().Type() == Type
}
