package tangle

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
)

// MessageTipSelectorEvents represents event happening on the tip-selector.
type MessageTipSelectorEvents struct {
	// Fired when a tip is added.
	TipAdded *events.Event
	// Fired when a tip is removed.
	TipRemoved *events.Event
}

func newMessageTipSelectorEvents() *MessageTipSelectorEvents {
	return &MessageTipSelectorEvents{
		TipAdded:   events.NewEvent(messageIDEvent),
		TipRemoved: events.NewEvent(messageIDEvent),
	}
}

// FactoryEvents represents events happening on a message factory.
type FactoryEvents struct {
	// Fired when a message is built including tips, sequence number and other metadata.
	MessageConstructed *events.Event
	// Fired when an error occurred.
	Error *events.Event
}

func newFactoryEvents() *FactoryEvents {
	return &FactoryEvents{
		MessageConstructed: events.NewEvent(messageConstructedEvent),
		Error:              events.NewEvent(events.ErrorCaller),
	}
}

// ParserEvents represents events happening on a message parser.
type ParserEvents struct {
	// Fired when a message was parsed.
	MessageParsed *events.Event
	// Fired when submitted bytes are rejected by a filter.
	BytesRejected *events.Event
	// Fired when a message got rejected by a filter.
	MessageRejected *events.Event
}

// MessageParsedEvent represents the parameters of messageParsedEvent
type MessageParsedEvent struct {
	Message *Message
	Peer    *peer.Peer
}

// BytesRejectedEvent represents the parameters of bytesRejectedEvent
type BytesRejectedEvent struct {
	Bytes []byte
	Peer  *peer.Peer
}

// MessageRejectedEvent represents the parameters of messageRejectedEvent
type MessageRejectedEvent struct {
	Message *Message
	Peer    *peer.Peer
}

func newParserEvents() *ParserEvents {
	return &ParserEvents{
		MessageParsed:   events.NewEvent(messageParsedEvent),
		BytesRejected:   events.NewEvent(bytesRejectedEvent),
		MessageRejected: events.NewEvent(messageRejectedEvent),
	}
}

// RequesterEvents represents events happening on a message requester.
type RequesterEvents struct {
	// Fired when a request for a given message should be sent.
	SendRequest *events.Event
	// MissingMessageAppeared is triggered when a message is actually present in the node's db although it was still being requested.
	MissingMessageAppeared *events.Event
}

// SendRequestEvent represents the parameters of sendRequestEvent
type SendRequestEvent struct {
	ID MessageID
}

// MissingMessageAppearedEvent represents the parameters of missingMessageAppearedEvent
type MissingMessageAppearedEvent struct {
	ID MessageID
}

func newRequesterEvents() *RequesterEvents {
	return &RequesterEvents{
		SendRequest:            events.NewEvent(sendRequestEvent),
		MissingMessageAppeared: events.NewEvent(missingMessageAppearedEvent),
	}
}

func sendRequestEvent(handler interface{}, params ...interface{}) {
	handler.(func(*SendRequestEvent))(params[0].(*SendRequestEvent))
}

func missingMessageAppearedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*MissingMessageAppearedEvent))(params[0].(*MissingMessageAppearedEvent))
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

func messageConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*Message))(params[0].(*Message))
}

func messageIDEvent(handler interface{}, params ...interface{}) {
	handler.(func(MessageID))(params[0].(MessageID))
}
