package networkdelay

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/mr-tron/base58"

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
	// PayloadName defines the name of the networkdelay payload.
	PayloadName = "networkdelay"
	payloadType = 189
)

// ID represents a 32 byte ID of a network delay payload.
type ID [32]byte

// String returns a human-friendly representation of the ID.
func (id ID) String() string {
	return base58.Encode(id[:])
}

// Payload represents the network delay payload type.
type Payload struct {
	model.Immutable[Payload, *Payload, payloadModel] `serix:"0"`
}

type payloadModel struct {
	ID       ID    `serix:"0"`
	SentTime int64 `serix:"1"` // [ns]

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewPayload creates a new  network delay payload.
func NewPayload(id ID, sentTime int64) *Payload {
	return model.NewImmutable[Payload](
		&payloadModel{
			ID:       id,
			SentTime: sentTime,
		},
	)
}

// ID returns the ID of the Payload.
func (p *Payload) ID() ID {
	return p.M.ID
}

// SentTime returns the type of the Payload.
func (p *Payload) SentTime() int64 {
	return p.M.SentTime
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type represents the identifier which addresses the network delay Payload type.
var Type = payload.NewType(payloadType, PayloadName)

// Type returns the type of the Payload.
func (p *Payload) Type() payload.Type {
	return Type
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
