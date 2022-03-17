package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/refactored/syncutils"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	ErrorEvent                *event.Event[error]
	TransactionProcessedEvent *event.Event[*TransactionProcessedEvent]

	*Storage
	*AvailabilityManager
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

func (l *Ledger) Setup() {
	// Attach = run async
	l.TransactionProcessedEvent.Attach(event.NewClosure[*TransactionProcessedEvent](l.ProcessFutureCone))
}

func (l *Ledger) ProcessTransaction(tx utxo.Transaction, meta *TransactionMetadata) (success bool, err error) {
	err = dataflow.New[*DataFlowParams](
		l.lockTransactionStep,
		l.CheckSolidity,
		/*
			l.ValidatePastCone,
			l.ExecuteTransaction,
			l.BookTransaction,
		*/
		l.triggerProcessedEventStep,
	).WithSuccessCallback(func(params *DataFlowParams) {
		success = true
	}).WithTerminationCallback(func(params *DataFlowParams) {
		l.Unlock(params.Transaction, false)
	}).Run(&DataFlowParams{
		Transaction:         tx,
		TransactionMetadata: meta,
	})

	if err != nil {
		// TODO: mark Transaction as invalid and trigger invalid event
		// eventually trigger generic errors if its not due to tx invalidity
		return false, err
	}

	return success, nil
}

func (l *Ledger) ProcessFutureCone(event *TransactionProcessedEvent) {
	// TODO: FILL WITH ACTUAL CONSUMERS
	_ = event.Inputs
	consumers := []utxo.TransactionID{}

	for consumersWalker := walker.New[utxo.TransactionID](true).PushAll(consumers...); consumersWalker.HasNext(); {
		transactionID := consumersWalker.Next()

		l.CachedTransactionMetadata(transactionID).Consume(func(consumerMetadata *TransactionMetadata) {
			l.CachedTransaction(transactionID).Consume(func(consumerTransaction utxo.Transaction) {
				if _, err := l.ProcessTransaction(consumerTransaction, consumerMetadata); err != nil {
					l.ErrorEvent.Trigger(errors.Errorf("failed to process Transaction with %s: %w", transactionID, err))
				}
			})
		})
	}
}

func (l *Ledger) StoreAndProcessTransaction(transaction utxo.Transaction) (success bool, err error) {
	cachedTransactionMetadata, stored := l.Store(transaction)
	defer cachedTransactionMetadata.Release()

	if !stored {
		return
	}

	l.TransactionStoredEvent.Trigger(&TransactionStoredEvent{&DataFlowParams{
		Transaction:         transaction,
		TransactionMetadata: nil, // todo retrieve
	}})

	return l.ProcessTransaction(transaction, cachedTransactionMetadata.Get())
}

func (l *Ledger) lockTransactionStep(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) error {
	l.Lock(params.Transaction, false)

	return next(params)
}

func (l *Ledger) triggerProcessedEventStep(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) (err error) {
	l.TransactionProcessedEvent.Trigger(&TransactionProcessedEvent{
		params,
	})

	return next(params)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type DataFlowParams struct {
	Transaction         utxo.Transaction
	TransactionMetadata *TransactionMetadata
	Inputs              []utxo.Output
}
