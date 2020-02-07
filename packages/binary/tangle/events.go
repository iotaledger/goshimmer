package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"
)

type Events struct {
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
	handler.(func(transaction.Id))(params[0].(transaction.Id))
}

func cachedTransactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *transactionmetadata.CachedTransactionMetadata))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*transactionmetadata.CachedTransactionMetadata).Retain().(*transactionmetadata.CachedTransactionMetadata),
	)
}
