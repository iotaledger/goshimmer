package faucet

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(Payload{}, serix.TypeSettings{}.WithObjectType(uint32(new(Payload).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*payload.Payload)(nil), new(Payload))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction as Payload interface: %w", err))
	}
}

const (
	// ObjectName defines the name of the faucet object (payload).
	ObjectName  = "faucet"
	payloadType = 2
)

// Payload represents a faucet request which contains an address for the faucet to send funds to.
type Payload struct {
	requestInner `serix:"0"`
}

type requestInner struct {
	PayloadType           payload.Type
	Address               ledgerstate.Address `serix:"1"`
	AccessManaPledgeID    identity.ID         `serix:"2"`
	ConsensusManaPledgeID identity.ID         `serix:"3"`
	Nonce                 uint64              `serix:"4"`
}

// RequestType represents the identifier for the faucet Payload type.
var (
	RequestType = payload.NewType(payloadType, ObjectName)
)

// NewRequest is the constructor of a Payload and creates a new Payload object from the given details.
func NewRequest(addr ledgerstate.Address, accessManaPledgeID, consensusManaPledgeID identity.ID, nonce uint64) *Payload {
	p := &Payload{
		requestInner{
			PayloadType:           RequestType,
			Address:               addr,
			AccessManaPledgeID:    accessManaPledgeID,
			ConsensusManaPledgeID: consensusManaPledgeID,
			Nonce:                 nonce,
		},
	}

	return p
}

// FromBytes parses the marshaled version of a Payload into a Go object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(data []byte) (payloadDecoded *Payload, consumedBytes int, err error) {
	payloadDecoded = new(Payload)

	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, payloadDecoded, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Request: %w", err)
		return
	}

	return
}

// RequestType returns the type of the faucet Payload.
func (p *Payload) Type() payload.Type {
	return RequestType
}

// Address returns the address of the faucet Payload.
func (p *Payload) Address() ledgerstate.Address {
	return p.requestInner.Address
}

// AccessManaPledgeID returns the access mana pledge ID of the faucet request.
func (p *Payload) AccessManaPledgeID() identity.ID {
	return p.requestInner.AccessManaPledgeID
}

// ConsensusManaPledgeID returns the consensus mana pledge ID of the faucet request.
func (p *Payload) ConsensusManaPledgeID() identity.ID {
	return p.requestInner.ConsensusManaPledgeID
}

// Bytes returns a marshaled version of the Payload.
func (p *Payload) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human readable version of faucet Payload payload (for debug purposes).
func (p *Payload) String() string {
	return stringify.Struct("FaucetPayload",
		stringify.StructField("address", p.Address().Base58()),
		stringify.StructField("accessManaPledgeID", p.AccessManaPledgeID().String()),
		stringify.StructField("consensusManaPledgeID", p.ConsensusManaPledgeID().String()),
	)
}

// IsFaucetReq checks if the message is faucet payload.
func IsFaucetReq(msg *tangle.Message) bool {
	return msg.Payload().Type() == RequestType
}
