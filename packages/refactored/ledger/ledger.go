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

func (l *Ledger) ProcessTransaction(transaction utxo.Transaction, metadata *TransactionMetadata) {
	err := l.SolidifyTransactionCommand()(&DataFlowParams{
		Transaction:         transaction,
		TransactionMetadata: metadata,
	})
	approversWalker := walker.New[utxo.TransactionID](true)
	for approversWalker.HasNext() {
		approversWalker.Next()
	}

	l.solidifyTransactionAndCollectApprovers

	_ = l.solidify()(&DataFlowParams{Transaction: transaction, TransactionMetadata: metadata}, nil)

	return
}

func (l *Ledger) solidify() (solidifyTransactionCommand dataflow.ChainedCommand[*DataFlowParams]) {

	return dataflow.New[*DataFlowParams](
		l.solidifyTransactionAndCollectApprovers(approversWalker),
		l.propagateSolidityToFutureCone(approversWalker),
	).ChainedCommand
}

func (l *Ledger) solidifyTransactionAndCollectApprovers(approversWalker *walker.Walker[utxo.TransactionID]) dataflow.ChainedCommand[*DataFlowParams] {
	return dataflow.New[*DataFlowParams](
		l.SolidifyTransactionCommand(),
		func(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) error {
			// queue parents
			approversWalker.PushAll()

			return next(params)
		},
	).ChainedCommand
}

func (l *Ledger) propagateSolidityToFutureCone(approversWalker *walker.Walker[utxo.TransactionID]) Command {
	return func(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) error {
		for approversWalker.HasNext() {
			l.CachedTransactionMetadata(approversWalker.Next()).Consume(func(consumerMetadata *TransactionMetadata) {
				l.CachedTransaction(consumerMetadata.ID()).Consume(func(consumerTransaction utxo.Transaction) {
					_ = l.solidifyTransactionAndCollectApprovers(approversWalker)(&DataFlowParams{
						Transaction:         consumerTransaction,
						TransactionMetadata: consumerMetadata,
					}, nil)
				})
			})
		}

		return next(params)
	}
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
