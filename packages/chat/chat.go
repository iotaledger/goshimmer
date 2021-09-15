package chat

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

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
	From       string
	FromLen    uint32
	To         string
	ToLen      uint32
	Message    string
	MessageLen uint32

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewPayload creates a new chat payload.
func NewPayload(from, to, message string) *Payload {
	return &Payload{
		From:       from,
		FromLen:    uint32(len([]byte(from))),
		To:         to,
		ToLen:      uint32(len([]byte(to))),
		Message:    message,
		MessageLen: uint32(len([]byte(message))),
	}
}

// FromBytes parses the marshaled version of a Payload into a Go object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse unmarshals an Payload using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Payload, err error) {
	// read information that are required to identify the payloa from the outside
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload size of chat payload: %w", err)
		return
	}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload type of chat payload: %w", err)
		return
	}

	// parse From
	result = &Payload{}
	fromLen, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse fromLen field of chat payload: %w", err)
		return
	}
	result.FromLen = fromLen

	from, err := marshalUtil.ReadBytes(int(fromLen))
	if err != nil {
		err = fmt.Errorf("failed to parse from field of chat payload: %w", err)
		return
	}
	result.From = string(from)

	// parse To
	toLen, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse toLen field of chat payload: %w", err)
		return
	}
	result.ToLen = toLen

	to, err := marshalUtil.ReadBytes(int(toLen))
	if err != nil {
		err = fmt.Errorf("failed to parse to field of chat payload: %w", err)
		return
	}
	result.To = string(to)

	// parse Message
	messageLen, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse messageLen field of chat payload: %w", err)
		return
	}
	result.MessageLen = messageLen

	message, err := marshalUtil.ReadBytes(int(messageLen))
	if err != nil {
		err = fmt.Errorf("failed to parse message field of chat payload: %w", err)
		return
	}
	result.Message = string(message)

	// store bytes, so we don't have to marshal manually
	consumedBytes := marshalUtil.ReadOffset()
	copy(result.bytes, marshalUtil.Bytes()[:consumedBytes])

	return result, nil
}

// Bytes returns a marshaled version of this Payload.
func (p *Payload) Bytes() (bytes []byte) {
	// acquire lock for reading bytes
	p.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = p.bytes; bytes != nil {
		p.bytesMutex.RUnlock()
		return
	}

	// switch to write lock
	p.bytesMutex.RUnlock()
	p.bytesMutex.Lock()
	defer p.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = p.bytes; bytes != nil {
		return
	}

	payloadLength := int(p.FromLen + p.ToLen + p.MessageLen + marshalutil.Uint32Size*3)
	// initialize helper
	marshalUtil := marshalutil.New(marshalutil.Uint32Size + marshalutil.Uint32Size + payloadLength)

	// marshal the payload specific information
	marshalUtil.WriteUint32(payload.TypeLength + uint32(payloadLength))
	marshalUtil.WriteBytes(Type.Bytes())
	marshalUtil.WriteUint32(p.FromLen)
	marshalUtil.WriteBytes([]byte(p.From))
	marshalUtil.WriteUint32(p.ToLen)
	marshalUtil.WriteBytes([]byte(p.To))
	marshalUtil.WriteUint32(p.MessageLen)
	marshalUtil.WriteBytes([]byte(p.Message))

	bytes = marshalUtil.Bytes()

	return bytes
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
var Type = payload.NewType(payloadType, PayloadName, func(data []byte) (payload payload.Payload, err error) {
	var consumedBytes int
	payload, consumedBytes, err = FromBytes(data)
	if err != nil {
		return nil, err
	}
	if consumedBytes != len(data) {
		return nil, errors.New("not all payload bytes were consumed")
	}
	return
})

// Type returns the type of the Payload.
func (p *Payload) Type() payload.Type {
	return Type
}
