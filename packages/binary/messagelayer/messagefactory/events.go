package messagefactory

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// Events represents events happening on a message factory.
type Events struct {
	// Fired when a message is built including tips, sequence number and other metadata.
	MessageConstructed *events.Event
	// Fired when an error occurred.
	Error *events.Event
}

func newEvents() *Events {
	return &Events{
		MessageConstructed: events.NewEvent(messageConstructedEvent),
		Error:              events.NewEvent(events.ErrorCaller),
	}
}

func messageConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*message.Message))(params[0].(*message.Message))
}
