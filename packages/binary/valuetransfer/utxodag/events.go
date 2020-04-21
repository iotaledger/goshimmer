package utxodag

import (
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"

	"github.com/iotaledger/hive.go/events"
)

type Events struct {
	// Get's called whenever a transaction
	TransactionReceived    *events.Event
	TransactionConflicting *events.Event
}

func newEvents() *Events {
	return &Events{
		TransactionReceived: events.NewEvent(cachedTransactionEvent),
	}
}

func cachedTransactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *CachedAttachment))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*CachedAttachment).Retain(),
	)
}
