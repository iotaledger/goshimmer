package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/syncutils"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	TransactionStoredEvent            *event.Event[utxo.TransactionID]
	TransactionSolidEvent             *event.Event[utxo.TransactionID]
	TransactionProcessedEvent         *event.Event[utxo.TransactionID]
	ConsumedTransactionProcessedEvent *event.Event[utxo.TransactionID]
	ErrorEvent                        *event.Event[error]

	*Storage
	*Solidifier
	*Validator
	*Executor
	*Utils
	*branchdag.BranchDAG

	syncutils.DAGMutex[[32]byte]

	vm utxo.VM
}

func New(store kvstore.KVStore, vm utxo.VM) (ledger *Ledger) {
	ledger = new(Ledger)
	ledger.ErrorEvent = event.New[error]()
	ledger.vm = vm
	ledger.Storage = NewStorage(ledger)
	ledger.Solidifier = NewSolidifier(ledger)

	return ledger
}

func (l *Ledger) Setup() {
	l.ConsumedTransactionProcessedEvent.Attach(event.NewClosure[utxo.TransactionID](func(txID utxo.TransactionID) {
		l.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
			l.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
				l.processTransaction(tx, txMetadata)
			})
		})
	}))
}

// StoreAndProcessTransaction is the only public facing api
func (l *Ledger) StoreAndProcessTransaction(tx utxo.Transaction) (processed bool) {
	cachedTransactionMetadata := l.CachedTransactionMetadata(tx.ID(), func(transactionID utxo.TransactionID) *TransactionMetadata {
		l.transactionStorage.Store(tx).Release()

		// TODO: STORE CONSUMERS

		processed = true
		return NewTransactionMetadata(transactionID)
	})

	// if we didn't store ourselves then we consider this call to be a success if this transaction was processed already
	//  (e.g. by a reattachment)
	if !processed {
		cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
			processed = metadata.Processed()
		})

		return processed
	}

	cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
		l.TransactionStoredEvent.Trigger(tx.ID())

		processed = l.processTransaction(tx, metadata)
	})

	return processed
}

func (l *Ledger) processTransaction(tx utxo.Transaction, txMeta *TransactionMetadata) (success bool) {
	if txMeta.Processed() {
		return false
	}

	l.DAGMutex.Lock(tx.ID())
	defer l.DAGMutex.Unlock(tx.ID())

	err := dataflow.New[*params](
		l.checkSolidityCommand,
		l.checkOutputsCausallyRelatedCommand,
		l.executeTransactionCommand,
		/*
			l.BookTransaction,
		*/
		l.notifyConsumersCommand,
	).WithSuccessCallback(func(params *params) {
		l.TransactionProcessedEvent.Trigger(params.Transaction.ID())

		success = true
	}).Run(&params{
		Transaction:         tx,
		TransactionMetadata: txMeta,
	})

	if err != nil {
		// TODO: mark Transaction as invalid and trigger invalid event
		// eventually trigger generic errors if its not due to tx invalidity
		return false
	}

	return success
}

func (l *Ledger) notifyConsumersCommand(params *params, next dataflow.Next[*params]) error {
	// TODO: FILL WITH ACTUAL CONSUMERS
	_ = params.Inputs
	consumers := []utxo.TransactionID{}

	for _, consumerTransactionId := range consumers {
		l.ConsumedTransactionProcessedEvent.Trigger(consumerTransactionId)
	}

	return next(params)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type params struct {
	Transaction         utxo.Transaction
	TransactionMetadata *TransactionMetadata
	Inputs              []utxo.Output
	InputsMetadata      map[utxo.OutputID]*OutputMetadata
	Outputs             []utxo.Output
}
