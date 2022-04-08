package ledger

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Ledger is an implementation of a "realities-ledger" that manages Outputs according to the principles of the
// quadruple-entry accounting. It acts as a wrapper for the underlying components and exposes the public facing API.
type Ledger struct {
	// Events is a dictionary for Ledger related events.
	Events *Events

	// Storage is a dictionary for storage related API endpoints.
	Storage *storage

	// Utils is a dictionary for utility methods that simplify the interaction with the Ledger.
	Utils *Utils

	// BranchDAG is a reference to the BranchDAG that is used by this Ledger.
	BranchDAG *branchdag.BranchDAG

	// dataFlow is a Ledger component that defines the data flow (how the different commands are chained together)
	dataFlow *dataFlow

	// validator is a Ledger component that bundles the API that is used to check the validity of a Transaction
	validator *validator

	// booker is a Ledger component that bundles the booking related API.
	booker *booker

	// options is a dictionary for configuration parameters of the Ledger.
	options *options

	// mutex is a DAGMutex that is used to make the Ledger thread safe.
	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

// New returns a new Ledger from the given options.
func New(options ...Option) (ledger *Ledger) {
	ledger = &Ledger{
		Events:  newEvents(),
		options: newOptions(options...),
		mutex:   syncutils.NewDAGMutex[utxo.TransactionID](),
	}

	ledger.BranchDAG = branchdag.New(branchdag.WithStore(ledger.options.store), branchdag.WithCacheTimeProvider(ledger.options.cacheTimeProvider))
	ledger.Storage = newStorage(ledger)
	ledger.validator = newValidator(ledger)
	ledger.booker = newBooker(ledger)
	ledger.dataFlow = newDataFlow(ledger)
	ledger.Utils = newUtils(ledger)

	ledger.Events.TransactionBooked.Attach(event.NewClosure[*TransactionBookedEvent](func(event *TransactionBookedEvent) {
		ledger.processConsumingTransactions(event.Outputs.IDs())
	}))

	return ledger
}

// CheckTransaction checks the validity of a Transaction.
func (l *Ledger) CheckTransaction(tx utxo.Transaction) (err error) {
	return l.dataFlow.checkTransaction().Run(newDataFlowParams(NewTransaction(tx)))
}

// StoreAndProcessTransaction stores and processes the given Transaction.
func (l *Ledger) StoreAndProcessTransaction(tx utxo.Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.storeAndProcessTransaction().Run(newDataFlowParams(NewTransaction(tx)))
}

// processTransaction tries to book a single Transaction.
func (l *Ledger) processTransaction(tx *Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.processTransaction().Run(newDataFlowParams(tx))
}

// processConsumingTransactions tries to book the transactions approving the given OutputIDs (it is used to propagate
// the booked status).
func (l *Ledger) processConsumingTransactions(outputIDs utxo.OutputIDs) {
	for it := l.Utils.UnprocessedConsumingTransactions(outputIDs).Iterator(); it.HasNext(); {
		go l.Storage.CachedTransaction(it.Next()).Consume(func(tx *Transaction) {
			_ = l.processTransaction(tx)
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
