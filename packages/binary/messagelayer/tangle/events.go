package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// Events represents events happening on the base layer Tangle.
type Events struct {
	// Fired when a message has been attached.
	MessageAttached *events.Event
	// Fired when a message has been solid, i.e. its past cone
	// is known and in the database.
	MessageSolid *events.Event
	// Fired when a message which was previously marked as missing was received.
	MissingMessageReceived *events.Event
	// Fired when a message is missing which is needed to solidify a given approver message.
	MessageMissing *events.Event
	// Fired when a message was missing for too long and is
	// therefore considered to be unsolidifiable.
	MessageUnsolidifiable *events.Event
	// Fired when a message was removed from storage.
	MessageRemoved *events.Event
}

type CachedMessage struct {
	Message         *message.CachedMessage
	MessageMetadata *CachedMessageMetadata
}

func newEvents() *Events {
	return &Events{
		MessageAttached:        events.NewEvent(cachedMessageEvent),
		MessageSolid:           events.NewEvent(cachedMessageEvent),
		MissingMessageReceived: events.NewEvent(cachedMessageEvent),
		MessageMissing:         events.NewEvent(messageIDEvent),
		MessageUnsolidifiable:  events.NewEvent(messageIDEvent),
		MessageRemoved:         events.NewEvent(messageIDEvent),
	}
}

func messageIDEvent(handler interface{}, params ...interface{}) {
	handler.(func(message.ID))(params[0].(message.ID))
}

func cachedMessageEvent(handler interface{}, params ...interface{}) {
	handler.(func(*CachedMessage))(cachedMessageRetain(params[0].(*CachedMessage)))
}

func cachedMessageRetain(object *CachedMessage) *CachedMessage {
	return &CachedMessage{
		Message:         object.Message.Retain(),
		MessageMetadata: object.MessageMetadata.Retain(),
	}
}
