package utxodag

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"

	"github.com/iotaledger/hive.go/events"
)

type Events struct {
	// Get's called whenever a transaction
	TransactionReceived *events.Event
	TransactionBooked   *events.Event
	Fork                *events.Event
}

func newEvents() *Events {
	return &Events{
		TransactionReceived: events.NewEvent(cachedTransactionEvent),
		TransactionBooked:   events.NewEvent(transactionBookedEvent),
		Fork:                events.NewEvent(forkEvent),
	}
}

func transactionBookedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *branchmanager.CachedBranch, []transaction.OutputId, bool))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*branchmanager.CachedBranch).Retain(),
		params[3].([]transaction.OutputId),
		params[4].(bool),
	)
}

func forkEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *branchmanager.CachedBranch, []transaction.OutputId))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*branchmanager.CachedBranch).Retain(),
		params[3].([]transaction.OutputId),
	)
}

func cachedTransactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *CachedAttachment))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*CachedAttachment).Retain(),
	)
}
