package chat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(Payload{}, serix.TypeSettings{}.WithObjectType(uint32(new(Payload).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering Chat type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*payload.Payload)(nil), new(Payload))
	if err != nil {
		panic(fmt.Errorf("error registering Chat as Payload interface: %w", err))
	}
}

// NewChat creates a new Chat.
func NewChat() *Chat {
	return &Chat{
		Events: Events{
			MessageReceived: events.NewEvent(chatEventCaller),
		},
	}
}

// Chat manages chats happening over the Tangle.
type Chat struct {
	Events
}

// Events define events occurring within a Chat.
type Events struct {
	// Fired when a chat message is received.
	MessageReceived *events.Event
}

// Event defines the information passed when a chat event fires.
type Event struct {
	From      string
	To        string
	Message   string
	Timestamp time.Time
	MessageID string
}

func chatEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Event))(params[0].(*Event))
}

const (
	// PayloadName defines the name of the chat payload.
	PayloadName = "chat"
	payloadType = 989
)

// Payload represents the chat payload type.
type Payload struct {
	From    string `serix:"0,lengthPrefixType=uint32"`
	To      string `serix:"1,lengthPrefixType=uint32"`
	Message string `serix:"2,lengthPrefixType=uint32"`

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewPayload creates a new chat payload.
func NewPayload(from, to, message string) *Payload {
	return &Payload{
		From:    from,
		To:      to,
		Message: message,
	}
}

// FromBytes parses the marshaled version of a Payload into a Go object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (payloadDecoded *Payload, consumedBytes int, err error) {
	payloadDecoded = new(Payload)

	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, payloadDecoded, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Chat Payload: %w", err)
		return
	}
	payloadDecoded.bytes = bytes

	return
}

// Bytes returns a marshaled version of this Payload.
func (p *Payload) Bytes() []byte {
	p.bytesMutex.Lock()
	defer p.bytesMutex.Unlock()
	if objBytes := p.bytes; objBytes != nil {
		return objBytes
	}

	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	p.bytes = objBytes
	return objBytes
}

// String returns a human-friendly representation of the Payload.
func (p *Payload) String() string {
	return stringify.Struct("ChatPayload",
		stringify.StructField("from", p.From),
		stringify.StructField("to", p.To),
		stringify.StructField("Message", p.Message),
	)
}

// Type represents the identifier which addresses the chat payload type.
var Type = payload.NewType(payloadType, PayloadName)

// Type returns the type of the Payload.
func (p *Payload) Type() payload.Type {
	return Type
}
