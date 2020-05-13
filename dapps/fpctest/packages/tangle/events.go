package tangle

import (
	"github.com/iotaledger/goshimmer/dapps/fpctest/packages/payload"
	"github.com/iotaledger/hive.go/events"
)

// Events is a container for the different kind of events of the Tangle.
type Events struct {
	// Get's called whenever a transaction
	PayloadAttached *events.Event
}

func newEvents() *Events {
	return &Events{
		PayloadAttached: events.NewEvent(cachedPayloadEvent),
	}
}

func payloadIDEvent(handler interface{}, params ...interface{}) {
	handler.(func(payload.ID))(params[0].(payload.ID))
}

func cachedPayloadEvent(handler interface{}, params ...interface{}) {
	handler.(func(*payload.CachedPayload, *CachedPayloadMetadata))(
		params[0].(*payload.CachedPayload).Retain(),
		params[1].(*CachedPayloadMetadata).Retain(),
	)
}
