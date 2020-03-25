package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"
)

type Events struct {
	// Get's called whenever a transaction
	TransactionAttached        *events.Event
	TransactionSolid           *events.Event
	MissingTransactionReceived *events.Event
	TransactionMissing         *events.Event
	TransactionUnsolidifiable  *events.Event
	TransactionRemoved         *events.Event
}

func newEvents() *Events {
	return &Events{
		TransactionAttached:        events.NewEvent(cachedTransactionEvent),
		TransactionSolid:           events.NewEvent(cachedTransactionEvent),
		MissingTransactionReceived: events.NewEvent(cachedTransactionEvent),
		TransactionMissing:         events.NewEvent(transactionIdEvent),
		TransactionUnsolidifiable:  events.NewEvent(transactionIdEvent),
		TransactionRemoved:         events.NewEvent(transactionIdEvent),
	}
}

func transactionIdEvent(handler interface{}, params ...interface{}) {
	handler.(func(message.Id))(params[0].(message.Id))
}

func cachedTransactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*message.CachedTransaction, *transactionmetadata.CachedTransactionMetadata))(
		params[0].(*message.CachedTransaction).Retain(),
		params[1].(*transactionmetadata.CachedTransactionMetadata).Retain(),
	)
}
