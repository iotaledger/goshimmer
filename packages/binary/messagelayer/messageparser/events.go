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

type MessageParsed struct {
	Message *message.Message
	Peer    *peer.Peer
}

type BytesRejected struct {
	Bytes []byte
	Err   error
	Peer  *peer.Peer
}

type MessageRejected struct {
	Message *message.Message
	Err     error
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
	handler.(func(*MessageParsed))(params[0].(*MessageParsed))
}

func bytesRejectedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*BytesRejected))(params[0].(*BytesRejected))
}

func messageRejectedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*MessageRejected))(params[0].(*MessageRejected))
}
