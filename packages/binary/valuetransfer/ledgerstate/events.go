package ledgerstate

import (
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"

	"github.com/iotaledger/hive.go/events"
)

type Events struct {
	// Get's called whenever a transaction
	TransactionReceived *events.Event
}

func newEvents() *Events {
	return &Events{
		TransactionReceived: events.NewEvent(cachedPayloadEvent),
	}
}

func cachedPayloadEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *CachedAttachment))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*CachedAttachment).Retain(),
	)
}
