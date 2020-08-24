package tipselector

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// Events represents event happening on the tip-selector.
type Events struct {
	// Fired when a tip is added.
	TipAdded *events.Event
	// Fired when a tip is removed.
	TipRemoved *events.Event
}

func newEvents() *Events {
	return &Events{
		TipAdded:   events.NewEvent(messageIDEvent),
		TipRemoved: events.NewEvent(messageIDEvent),
	}
}

func messageIDEvent(handler interface{}, params ...interface{}) {
	handler.(func(message.ID))(params[0].(message.ID))
}
