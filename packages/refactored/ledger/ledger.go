package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/refactored/syncutils"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	ErrorEvent *event.Event[error]

	*Storage
	*AvailabilityManager
	*DataFlow
	syncutils.DAGMutex[[32]byte]

	vm utxo.VM
}

func New(store kvstore.KVStore, vm utxo.VM) (ledger *Ledger) {
	ledger = new(Ledger)
	ledger.ErrorEvent = event.New[error]()
	ledger.vm = vm
	ledger.Storage = NewStorage(ledger)
	ledger.AvailabilityManager = NewAvailabilityManager(ledger)

	return ledger
}

func (l *Ledger) StoreAndProcessTransaction(transaction utxo.Transaction) {
	cachedTransactionMetadata, stored := l.Store(transaction)
	defer cachedTransactionMetadata.Release()

	if !stored {
		return
	}

	l.TransactionStoredEvent.Trigger(&TransactionStoredEvent{
		Transaction:         transaction,
		TransactionMetadata: cachedTransactionMetadata.Get(),
	})

	l.ProcessTransaction(transaction, cachedTransactionMetadata.Get())
}

func (l *Ledger) ProcessTransaction(tx utxo.Transaction, meta *TransactionMetadata) {
	success, consumers, err := l.SolidifyTransaction(tx, meta)
	if !success {
		return
	}

	// TODO: async event
	consumersWalker := walker.New[utxo.TransactionID](true).PushAll(consumers...)
	for consumersWalker.HasNext() {
		l.CachedTransactionMetadata(consumersWalker.Next()).Consume(func(consumerMetadata *TransactionMetadata) {
			l.CachedTransaction(consumerMetadata.ID()).Consume(func(consumerTransaction utxo.Transaction) {
				_, consumers, err := l.SolidifyTransaction(tx, meta)

			})
		})

		consumersWalker.Next()
	}

	return
}

func (l *Ledger) forkSingleTransactionCommand() (solidificationCommand dataflow.ChainedCommand[*DataFlowParams]) {
	return l.LockedCommand(
		dataflow.New[*DataFlowParams](
			l.ForkTransaction,
		).WithErrorCallback(func(err error, params *DataFlowParams) {
			// trigger generic errors if its not due to tx invalidity
		}).ChainedCommand,
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type DataFlowParams struct {
	Transaction         utxo.Transaction
	TransactionMetadata *TransactionMetadata
	Inputs              []utxo.Output
}
