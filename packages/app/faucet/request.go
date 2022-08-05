package faucet

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
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
	model.Immutable[Payload, *Payload, requestModel] `serix:"0"`
}

type requestModel struct {
	PayloadType           payload.Type
	Address               devnetvm.Address `serix:"1"`
	AccessManaPledgeID    identity.ID      `serix:"2"`
	ConsensusManaPledgeID identity.ID      `serix:"3"`
	Nonce                 uint64           `serix:"4"`
}

// RequestType represents the identifier for the faucet Payload type.
var (
	RequestType = payload.NewType(payloadType, ObjectName)
)

// NewRequest is the constructor of a Payload and creates a new Payload object from the given details.
func NewRequest(addr devnetvm.Address, accessManaPledgeID, consensusManaPledgeID identity.ID, nonce uint64) *Payload {
	p := model.NewImmutable[Payload](
		&requestModel{
			PayloadType:           RequestType,
			Address:               addr,
			AccessManaPledgeID:    accessManaPledgeID,
			ConsensusManaPledgeID: consensusManaPledgeID,
			Nonce:                 nonce,
		},
	)

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

// Type returns the type of the faucet Payload.
func (p *Payload) Type() payload.Type {
	return RequestType
}

// Address returns the address of the faucet Payload.
func (p *Payload) Address() devnetvm.Address {
	return p.M.Address
}

// AccessManaPledgeID returns the access mana pledge ID of the faucet request.
func (p *Payload) AccessManaPledgeID() identity.ID {
	return p.M.AccessManaPledgeID
}

// ConsensusManaPledgeID returns the consensus mana pledge ID of the faucet request.
func (p *Payload) ConsensusManaPledgeID() identity.ID {
	return p.M.ConsensusManaPledgeID
}

// SetAccessManaPledgeID sets the access mana pledge ID of the faucet request.
func (p *Payload) SetAccessManaPledgeID(id identity.ID) {
	p.M.AccessManaPledgeID = id
}

// SetConsensusManaPledgeID sets the consensus mana pledge ID of the faucet request.
func (p *Payload) SetConsensusManaPledgeID(id identity.ID) {
	p.M.ConsensusManaPledgeID = id
}

// IsFaucetReq checks if the block is faucet payload.
func IsFaucetReq(blk *tangleold.Block) bool {
	return blk.Payload().Type() == RequestType
}
