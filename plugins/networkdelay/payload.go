package networkdelay

import (
	"context"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"

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
	payloadInner `serix:"0"`
}

type payloadInner struct {
	ID       ID    `serix:"0"`
	SentTime int64 `serix:"1"` // [ns]

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewPayload creates a new  network delay payload.
func NewPayload(id ID, sentTime int64) *Payload {
	return &Payload{
		payloadInner{
			ID:       id,
			SentTime: sentTime,
		},
	}
}

// FromBytes parses the marshaled version of a Payload into a Go object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (payload *Payload, consumedBytes int, err error) {
	payload = new(Payload)

	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, payload, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse NetworkDelayPayload: %w", err)
		return
	}

	return
}

// Bytes returns a marshaled version of this Payload.
func (p *Payload) Bytes() []byte {
	p.bytesMutex.Lock()
	defer p.bytesMutex.Unlock()
	if objBytes := p.payloadInner.bytes; objBytes != nil {
		return objBytes
	}

	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	p.payloadInner.bytes = objBytes
	return objBytes
}

// String returns a human-friendly representation of the Payload.
func (p *Payload) String() string {
	return stringify.Struct("NetworkDelayPayload",
		stringify.StructField("id", p.ID),
		stringify.StructField("sentTime", uint64(p.SentTime)),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type represents the identifier which addresses the network delay Payload type.
var Type = payload.NewType(payloadType, PayloadName)

// Type returns the type of the Payload.
func (p *Payload) Type() payload.Type {
	return Type
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
