package chat

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const (
	// ObjectName defines the name of the chat object.
	ObjectName = "chat"
)

// Payload represents the chat object type.
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

// NewPayload creates a new chat object.
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

// FromBytes parses the marshaled version of an Object into a Go object.
// It either returns a new Object or fills an optionally provided Object with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse unmarshals an Object using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Payload, err error) {
	// read information that are required to identify the object from the outside
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

	return
}

// Bytes returns a marshaled version of this Object.
func (o *Payload) Bytes() (bytes []byte) {
	// acquire lock for reading bytes
	o.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = o.bytes; bytes != nil {
		o.bytesMutex.RUnlock()
		return
	}

	// switch to write lock
	o.bytesMutex.RUnlock()
	o.bytesMutex.Lock()
	defer o.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = o.bytes; bytes != nil {
		return
	}

	payloadLength := int(o.FromLen + o.ToLen + o.MessageLen + marshalutil.Uint32Size*3)
	// initialize helper
	marshalUtil := marshalutil.New(marshalutil.Uint32Size + marshalutil.Uint32Size + payloadLength)

	// marshal the payload specific information
	marshalUtil.WriteUint32(payload.TypeLength + uint32(payloadLength))
	marshalUtil.WriteBytes(Type.Bytes())
	marshalUtil.WriteUint32(o.FromLen)
	marshalUtil.WriteBytes([]byte(o.From))
	marshalUtil.WriteUint32(o.ToLen)
	marshalUtil.WriteBytes([]byte(o.To))
	marshalUtil.WriteUint32(o.MessageLen)
	marshalUtil.WriteBytes([]byte(o.Message))

	bytes = marshalUtil.Bytes()

	return
}

// String returns a human-friendly representation of the Object.
func (o *Payload) String() string {
	return stringify.Struct("ChatPayload",
		stringify.StructField("from", o.From),
		stringify.StructField("to", o.To),
		stringify.StructField("Message", o.Message),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type represents the identifier which addresses the network delay Object type.
var Type = payload.NewType(989, ObjectName, func(data []byte) (payload payload.Payload, err error) {
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

// Type returns the type of the Object.
func (o *Payload) Type() payload.Type {
	return Type
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
