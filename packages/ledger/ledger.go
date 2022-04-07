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
	Events    *Events
	Storage   *storage
	Utils     *Utils
	BranchDAG *branchdag.BranchDAG

	dataFlow  *dataFlow
	validator *validator
	booker    *booker
	options   *options
	mutex     *syncutils.DAGMutex[utxo.TransactionID]
}

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

func (l *Ledger) CheckTransaction(tx utxo.Transaction) (err error) {
	return l.dataFlow.checkTransaction().Run(newDataFlowParams(NewTransaction(tx)))
}

func (l *Ledger) StoreAndProcessTransaction(tx utxo.Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.storeAndProcessTransaction().Run(newDataFlowParams(NewTransaction(tx)))
}

func (l *Ledger) processTransaction(tx *Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.processTransaction().Run(newDataFlowParams(tx))
}

func (l *Ledger) processConsumingTransactions(outputIDs utxo.OutputIDs) {
	for it := l.Utils.UnprocessedConsumingTransactions(outputIDs).Iterator(); it.HasNext(); {
		consumingTransactionID := it.Next()
		go l.Storage.CachedTransaction(consumingTransactionID).Consume(func(tx *Transaction) {
			_ = l.processTransaction(tx)
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
