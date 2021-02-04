package fcob

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/events"
)

// Events defines all the events related to the opinion manager.
type Events struct {
	// Fired when an opinion of a payload is formed.
	PayloadOpinionFormed *events.Event
}

// PayloadOpinionFormedEvent holds data about a PayloadOpinionFormed event.
type PayloadOpinionFormedEvent struct {
	// The messageID of the message containing the payload.
	MessageID tangle.MessageID
	// The opinion of the payload.
	Opinion *Opinion
}

func payloadOpinionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*PayloadOpinionFormedEvent))(params[0].(*PayloadOpinionFormedEvent))
}
