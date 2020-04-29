package utxodag

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"

	"github.com/iotaledger/hive.go/events"
)

// Events is a container for the different kind of events of the UTXODAG.
type Events struct {
	// TransactionReceived gets triggered whenever a transaction was received for the first time (not solid yet).
	TransactionReceived *events.Event

	// TransactionBooked gets triggered whenever a transactions becomes solid and gets booked into a particular branch.
	TransactionBooked *events.Event

	// Fork gets triggered when a previously un-conflicting transaction get's some of its inputs double spend, so that a
	// new Branch is created.
	Fork *events.Event
}

func newEvents() *Events {
	return &Events{
		TransactionReceived: events.NewEvent(cachedTransactionEvent),
		TransactionBooked:   events.NewEvent(transactionBookedEvent),
		Fork:                events.NewEvent(forkEvent),
	}
}

func transactionBookedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *branchmanager.CachedBranch, []transaction.OutputID, bool))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*branchmanager.CachedBranch).Retain(),
		params[3].([]transaction.OutputID),
		params[4].(bool),
	)
}

func forkEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *branchmanager.CachedBranch, []transaction.OutputID))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*branchmanager.CachedBranch).Retain(),
		params[3].([]transaction.OutputID),
	)
}

func cachedTransactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *CachedAttachment))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*CachedAttachment).Retain(),
	)
}
