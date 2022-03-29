package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/syncutils"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	TransactionStoredEvent            *event.Event[TransactionID]
	TransactionSolidEvent             *event.Event[TransactionID]
	TransactionProcessedEvent         *event.Event[TransactionID]
	ConsumedTransactionProcessedEvent *event.Event[TransactionID]
	ErrorEvent                        *event.Event[error]

	*Options
	*Storage
	*Solidifier
	*Validator
	*Executor
	*Booker
	*Utils

	vm VM

	*branchdag.BranchDAG
	*syncutils.DAGMutex[[32]byte]
}

func New(store kvstore.KVStore, vm VM, options ...Option) (ledger *Ledger) {
	ledger = &Ledger{
		TransactionStoredEvent:            event.New[TransactionID](),
		TransactionSolidEvent:             event.New[TransactionID](),
		TransactionProcessedEvent:         event.New[TransactionID](),
		ConsumedTransactionProcessedEvent: event.New[TransactionID](),
		ErrorEvent:                        event.New[error](),

		BranchDAG: branchdag.NewBranchDAG(store, database.NewCacheTimeProvider(0)),
		DAGMutex:  syncutils.NewDAGMutex[[32]byte](),

		vm: vm,
	}

	ledger.Configure(options...)
	ledger.Storage = NewStorage(ledger)
	ledger.Solidifier = NewSolidifier(ledger)
	ledger.Validator = NewValidator(ledger)
	ledger.Executor = NewExecutor(ledger)
	ledger.Booker = NewBooker(ledger)
	ledger.Utils = NewUtils(ledger)

	return ledger
}

// Configure modifies the configuration of the Ledger.
func (l *Ledger) Configure(options ...Option) {
	if l.Options == nil {
		l.Options = &Options{
			Store:              mapdb.NewMapDB(),
			CacheTimeProvider:  database.NewCacheTimeProvider(0),
			LazyBookingEnabled: true,
		}
	}

	for _, option := range options {
		option(l.Options)
	}
}

func (l *Ledger) Setup() {
	l.ConsumedTransactionProcessedEvent.Attach(event.NewClosure[TransactionID](func(txID TransactionID) {
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

	cachedTransactionMetadata := l.CachedTransactionMetadata(transaction.ID(), func(transactionID TransactionID) *TransactionMetadata {
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
		l.ErrorEvent.Trigger(err)

		// TODO: mark Transaction as invalid and trigger invalid event
		return false
	}

	return success
}

func (l *Ledger) notifyConsumersCommand(params *params, next dataflow.Next[*params]) error {
	// TODO: FILL WITH ACTUAL CONSUMERS
	_ = params.Inputs
	var consumers []TransactionID

	for _, consumerTransactionId := range consumers {
		l.ConsumedTransactionProcessedEvent.Trigger(consumerTransactionId)
	}

	return next(params)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type params struct {
	Transaction         *Transaction
	TransactionMetadata *TransactionMetadata
	InputIDs            OutputIDs
	Inputs              Outputs
	InputsMetadata      OutputsMetadata
	Consumers           []*Consumer
	Outputs             Outputs
	OutputsMetadata     OutputsMetadata
}
