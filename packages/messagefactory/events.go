package messagefactory

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

var Events = struct {
	// A PayloadConstructed event is triggered when a message's payload is built.
	// Each ontology should implement a PayloadBuilder which triggers PayloadConstructed.
	PayloadConstructed *events.Event
	// A MessageConstructed event is triggered when a message is built including tips, sequence number and other metadata.
	MessageConstructed *events.Event
}{
	PayloadConstructed: events.NewEvent(payloadConstructedEvent),
	MessageConstructed: events.NewEvent(messageConstructedEvent),
}

func payloadConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(payload.Payload))(params[0].(payload.Payload))
}

func messageConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*message.Message))(params[0].(*message.Message))
}
