package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

type Events struct {
	// Get's called whenever a transaction
	PayloadAttached        *events.Event
	PayloadSolid           *events.Event
	MissingPayloadReceived *events.Event
	PayloadMissing         *events.Event
	PayloadUnsolidifiable  *events.Event
	TransactionRemoved     *events.Event
	OutputMissing          *events.Event

	TransactionSolid *events.Event
}

func newEvents() *Events {
	return &Events{
		PayloadAttached:        events.NewEvent(cachedPayloadEvent),
		PayloadSolid:           events.NewEvent(cachedPayloadEvent),
		MissingPayloadReceived: events.NewEvent(cachedPayloadEvent),
		PayloadMissing:         events.NewEvent(payloadIdEvent),
		PayloadUnsolidifiable:  events.NewEvent(payloadIdEvent),
		OutputMissing:          events.NewEvent(outputIdEvent),

		TransactionSolid: events.NewEvent(transactionEvent),
	}
}

func payloadIdEvent(handler interface{}, params ...interface{}) {
	handler.(func(payload.Id))(params[0].(payload.Id))
}

func cachedPayloadEvent(handler interface{}, params ...interface{}) {
	handler.(func(*payload.CachedPayload, *CachedPayloadMetadata))(
		params[0].(*payload.CachedPayload).Retain(),
		params[1].(*CachedPayloadMetadata).Retain(),
	)
}

func transactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.Transaction, *CachedTransactionMetadata))(
		params[0].(*transaction.Transaction),
		params[1].(*CachedTransactionMetadata).Retain(),
	)
}

func outputIdEvent(handler interface{}, params ...interface{}) {
	handler.(func(transaction.OutputId))(params[0].(transaction.OutputId))
}
