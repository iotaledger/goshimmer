package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
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
	*Booker
	*Utils
	*branchdag.BranchDAG

	syncutils.DAGMutex[[32]byte]
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
			l.CachedTransaction(txID).Consume(func(tx *Transaction) {
				l.processTransaction(tx, txMetadata)
			})
		})
	}))
}

// StoreAndProcessTransaction is the only public facing api
func (l *Ledger) StoreAndProcessTransaction(tx utxo.Transaction) (processed bool) {
	transaction := NewTransaction(tx)

	cachedTransactionMetadata := l.CachedTransactionMetadata(transaction.ID(), func(transactionID utxo.TransactionID) *TransactionMetadata {
		l.transactionStorage.Store(transaction).Release()
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
		l.TransactionStoredEvent.Trigger(transaction.ID())

		processed = l.processTransaction(transaction, metadata)
	})

	return processed
}

func (l *Ledger) processTransaction(tx *Transaction, txMeta *TransactionMetadata) (success bool) {
	l.Lock(tx.ID())
	defer l.Unlock(tx.ID())

	if txMeta.Processed() {
		return true
	}

	err := dataflow.New[*params](
		l.checkSolidityCommand,
		l.checkOutputsCausallyRelatedCommand,
		l.executeTransactionCommand,
		l.bookTransactionCommand,
		l.notifyConsumersCommand,
	).WithSuccessCallback(func(params *params) {
		generics.ForEach(params.Consumers, func(consumer *Consumer) {
			consumer.SetProcessed()
		})

		txMeta.SetProcessed(true)

		l.TransactionProcessedEvent.Trigger(tx.ID())

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
	Transaction         *Transaction
	TransactionMetadata *TransactionMetadata
	InputsIDs           []utxo.OutputID
	Inputs              []*Output
	InputsMetadata      map[utxo.OutputID]*OutputMetadata
	Consumers           []*Consumer
	Outputs             []*Output
	OutputsMetadata     map[utxo.OutputID]*OutputMetadata
}
