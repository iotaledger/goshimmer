package faucet

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const (
	// ObjectName defines the name of the faucet object (payload).
	ObjectName  = "faucet"
	payloadType = 2
)

// Request represents a faucet request which contains an address for the faucet to send funds to.
type Request struct {
	payloadType           payload.Type
	address               ledgerstate.Address
	accessManaPledgeID    identity.ID
	consensusManaPledgeID identity.ID
	nonce                 uint64
}

// Type represents the identifier for the faucet Request type.
var (
	Type = payload.NewType(payloadType, ObjectName, PayloadUnmarshaler)
)

// NewRequest is the constructor of a Request and creates a new Request object from the given details.
func NewRequest(addr ledgerstate.Address, accessManaPledgeID, consensusManaPledgeID identity.ID, nonce uint64) *Request {
	p := &Request{
		payloadType:           Type,
		address:               addr,
		accessManaPledgeID:    accessManaPledgeID,
		consensusManaPledgeID: consensusManaPledgeID,
		nonce:                 nonce,
	}

	return p
}

// FromBytes parses the marshaled version of a Request into a request object.
func FromBytes(bytes []byte) (result *Request, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	result = &Request{}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = errors.Errorf("failed to parse payload size (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.payloadType, err = payload.TypeFromMarshalUtil(marshalUtil)
	if err != nil {
		err = errors.Errorf("failed to parse Type from MarshalUtil: %w", err)
		return
	}
	addr, err := marshalUtil.ReadBytes(ledgerstate.AddressLength)
	if err != nil {
		err = errors.Errorf("failed to parse address of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.address, _, err = ledgerstate.AddressFromBytes(addr)
	if err != nil {
		err = errors.Errorf("failed to unmarshal address of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.accessManaPledgeID, err = identity.IDFromMarshalUtil(marshalUtil)
	if err != nil {
		err = errors.Errorf("failed to unmarshal access mana pledge ID of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.consensusManaPledgeID, err = identity.IDFromMarshalUtil(marshalUtil)
	if err != nil {
		err = errors.Errorf("failed to unmarshal consensus mana pledge ID of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.nonce, err = marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to unmarshal nonce of faucet request (%v): %w", err, cerrors.ErrParseBytesFailed)
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
func (p *Request) Address() ledgerstate.Address {
	return p.address
}

// AccessManaPledgeID returns the access mana pledge ID of the faucet request.
func (p *Request) AccessManaPledgeID() identity.ID {
	return p.accessManaPledgeID
}

// ConsensusManaPledgeID returns the consensus mana pledge ID of the faucet request.
func (p *Request) ConsensusManaPledgeID() identity.ID {
	return p.consensusManaPledgeID
}

// Bytes marshals the faucet Request payload into a sequence of bytes.
func (p *Request) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(payload.TypeLength + uint32(ledgerstate.AddressLength+identity.IDLength+identity.IDLength+pow.NonceBytes))
	marshalUtil.WriteBytes(p.Type().Bytes())
	marshalUtil.WriteBytes(p.address.Bytes())
	marshalUtil.WriteBytes(p.accessManaPledgeID.Bytes())
	marshalUtil.WriteBytes(p.consensusManaPledgeID.Bytes())
	marshalUtil.WriteUint64(p.nonce)

	// return result
	return marshalUtil.Bytes()
}

// String returns a human readable version of faucet Request payload (for debug purposes).
func (p *Request) String() string {
	return stringify.Struct("FaucetPayload",
		stringify.StructField("address", p.Address().Base58()),
		stringify.StructField("accessManaPledgeID", p.accessManaPledgeID.String()),
		stringify.StructField("consensusManaPledgeID", p.consensusManaPledgeID.String()),
	)
}

// PayloadUnmarshaler sets the generic unmarshaler.
func PayloadUnmarshaler(data []byte) (payload payload.Payload, err error) {
	var consumedBytes int
	payload, consumedBytes, err = FromBytes(data)
	if err != nil {
		return nil, err
	}
	if consumedBytes != len(data) {
		return nil, errors.New("not all payload bytes were consumed")
	}
	return
}

// IsFaucetReq checks if the message is faucet payload.
func IsFaucetReq(msg *tangle.Message) bool {
	return msg.Payload().Type() == Type
}
