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
	l.TransactionProcessedEvent.Attach(event.NewClosure[*TransactionProcessedEvent](l.processFutureCone))

	// attach sync = run in scope of event (while locks are still held)
	l.TransactionStoredEvent.AttachSync(event.NewClosure[*TransactionStoredEvent](l.processStoredTransaction))
}

// StoreAndProcessTransaction is the only public facing api
func (l *Ledger) StoreAndProcessTransaction(transaction utxo.Transaction) (success bool, err error) {
	// use computeifabsent as a mutex to only store things once
	cachedTransactionMetadata := l.CachedTransactionMetadata(transaction.ID(), func(transactionID utxo.TransactionID) *TransactionMetadata {
		l.transactionStorage.Store(transaction).Release()

		// TODO: STORE CONSUMERS

		success = true
		return NewTransactionMetadata(transactionID)
	})

	// if we didn't store ourselves then we consider this call to be a success if this transaction was processed already
	// before (e.g. by a reattachment)
	// TODO: maybe rename solid to "processed"
	if !success {
		cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
			success = metadata.Solid()
		})

		return success, nil
	}

	cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
		l.TransactionStoredEvent.Trigger(&TransactionStoredEvent{&DataFlowParams{
			Transaction:         transaction,
			TransactionMetadata: metadata,
		}})

		success = metadata.solid
	})

	return success, nil
}

func (l *Ledger) processTransaction(tx utxo.Transaction, meta *TransactionMetadata) (success bool, err error) {
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
		// TODO: fill consumers from outputs
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

func (l *Ledger) processFutureCone(event *TransactionProcessedEvent) {
	// TODO: FILL WITH ACTUAL CONSUMERS
	_ = event.Inputs
	consumers := []utxo.TransactionID{}

	for consumersWalker := walker.New[utxo.TransactionID](true).PushAll(consumers...); consumersWalker.HasNext(); {
		transactionID := consumersWalker.Next()

		l.CachedTransactionMetadata(transactionID).Consume(func(consumerMetadata *TransactionMetadata) {
			l.CachedTransaction(transactionID).Consume(func(consumerTransaction utxo.Transaction) {
				if _, err := l.processTransaction(consumerTransaction, consumerMetadata); err != nil {
					l.ErrorEvent.Trigger(errors.Errorf("failed to process Transaction with %s: %w", transactionID, err))
				}
			})
		})
	}
}

func (l *Ledger) processStoredTransaction(event *TransactionStoredEvent) {
	if _, err := l.processTransaction(event.Transaction, event.TransactionMetadata); err != nil {
		l.ErrorEvent.Trigger(errors.Errorf("failed to process stored Transaction with %s: %w", event.Transaction.ID(), err))
	}
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
