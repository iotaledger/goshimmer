package messagerequester

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/hive.go/events"
)

// Events represents events happening on a message requester.
type Events struct {
	// Fired when a request for a given message should be sent.
	SendRequest *events.Event
	// MissingMessageAppeared is triggered when a message is actually present in the node's db although it was still being requested.
	MissingMessageAppeared *events.Event
}

// SendRequestEvent represents the parameters of sendRequestEvent
type SendRequestEvent struct {
	ID message.ID
}

// MissingMessageAppearedEvent represents the parameters of missingMessageAppearedEvent
type MissingMessageAppearedEvent struct {
	ID message.ID
}

func newEvents() *Events {
	return &Events{
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
