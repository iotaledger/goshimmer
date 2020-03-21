package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
)

type Events struct {
	// Get's called whenever a transaction
	PayloadAttached        *events.Event
	PayloadSolid           *events.Event
	MissingPayloadReceived *events.Event
	PayloadMissing         *events.Event
	PayloadUnsolidifiable  *events.Event
	TransactionRemoved     *events.Event
}

func newEvents() *Events {
	return &Events{
		PayloadAttached:        events.NewEvent(cachedPayloadEvent),
		PayloadSolid:           events.NewEvent(cachedPayloadEvent),
		MissingPayloadReceived: events.NewEvent(cachedPayloadEvent),
		PayloadMissing:         events.NewEvent(payloadIdEvent),
		PayloadUnsolidifiable:  events.NewEvent(payloadIdEvent),
		TransactionRemoved:     events.NewEvent(payloadIdEvent),
	}
}

func payloadIdEvent(handler interface{}, params ...interface{}) {
	handler.(func(payload.Id))(params[0].(payload.Id))
}

func cachedPayloadEvent(handler interface{}, params ...interface{}) {
	handler.(func(*payload.CachedPayload, *payload.CachedMetadata))(
		params[0].(*payload.CachedPayload).Retain(),
		params[1].(*payload.CachedMetadata).Retain(),
	)
}
