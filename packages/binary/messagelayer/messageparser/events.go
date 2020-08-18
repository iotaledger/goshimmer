package messageparser

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events represents events happening on a message parser.
type Events struct {
	// Fired when a message was parsed.
	MessageParsed *events.Event
	// Fired when submitted bytes are rejected by a filter.
	BytesRejected *events.Event
	// Fired when a message got rejected by a filter.
	MessageRejected *events.Event
}

// MessageParsedEvent represents the parameters of messageParsedEvent
type MessageParsedEvent struct {
	Message *message.Message
	Peer    *peer.Peer
}

// BytesRejectedEvent represents the parameters of bytesRejectedEvent
type BytesRejectedEvent struct {
	Bytes []byte
	Peer  *peer.Peer
}

// MessageRejectedEvent represents the parameters of messageRejectedEvent
type MessageRejectedEvent struct {
	Message *message.Message
	Peer    *peer.Peer
}

func newEvents() *Events {
	return &Events{
		MessageParsed:   events.NewEvent(messageParsedEvent),
		BytesRejected:   events.NewEvent(bytesRejectedEvent),
		MessageRejected: events.NewEvent(messageRejectedEvent),
	}
}

func messageParsedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*MessageParsedEvent))(params[0].(*MessageParsedEvent))
}

func bytesRejectedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*BytesRejectedEvent, error))(params[0].(*BytesRejectedEvent), params[1].(error))
}

func messageRejectedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*MessageRejectedEvent, error))(params[0].(*MessageRejectedEvent), params[1].(error))
}
