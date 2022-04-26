package chat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

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
func FromBytes(bytes []byte) (payload *Payload, consumedBytes int, err error) {
	payload = new(Payload)

	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, payload, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Chat Payload: %w", err)
		return
	}
	payload.bytes = bytes

	return
}

// FromBytes parses the marshaled version of a Payload into a Go object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytesOld(bytes []byte) (result *Payload, consumedBytes int, err error) {
	//TODO: remove eventually
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()
	result.bytes = bytes
	return
}

// Parse unmarshals an Payload using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Payload, err error) {
	// read information that are required to identify the payloa from the outside
	//if _, err = marshalUtil.ReadUint32(); err != nil {
	//	err = fmt.Errorf("failed to parse payload size of chat payload: %w", err)
	//	return
	//}
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

// Bytes returns a marshaled version of this Payload.
func (p *Payload) BytesOld() (bytes []byte) {
	//TODO: remove eventually

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

	// initialize helper
	marshalUtil := marshalutil.New(marshalutil.Uint32Size + marshalutil.Uint32Size)

	// marshal the payload specific information
	marshalUtil.WriteBytes(Type.Bytes())
	marshalUtil.WriteUint32(uint32(len(p.From)))
	marshalUtil.WriteBytes([]byte(p.From))
	marshalUtil.WriteUint32(uint32(len(p.To)))
	marshalUtil.WriteBytes([]byte(p.To))
	marshalUtil.WriteUint32(uint32(len(p.Message)))
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
