package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/syncutils"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	TransactionStoredEvent            *event.Event[TransactionID]
	TransactionBookedEvent            *event.Event[TransactionID]
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
		TransactionBookedEvent:            event.New[TransactionID](),
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
				_ = l.processTransaction(tx, txMetadata)
			})
		})
	}))
}

// StoreAndProcessTransaction is the only public facing api
func (l *Ledger) StoreAndProcessTransaction(tx utxo.Transaction) (err error) {
	l.Lock(tx.ID())
	defer l.Unlock(tx.ID())

	return l.storeAndProcessTransactionDataFlow().Run(&params{
		Transaction: NewTransaction(tx),
		InputIDs:    l.resolveInputs(tx.Inputs()),
	})
}

func (l *Ledger) CheckTransaction(tx utxo.Transaction) (err error) {
	return l.checkTransactionDataFlow().Run(&params{
		Transaction: NewTransaction(tx),
		InputIDs:    l.resolveInputs(tx.Inputs()),
	})
}

func (l *Ledger) processTransaction(tx *Transaction, txMetadata *TransactionMetadata) (err error) {
	l.Lock(tx.ID())
	defer l.Unlock(tx.ID())

	return l.processTransactionDataFlow().Run(&params{
		Transaction:         tx,
		TransactionMetadata: txMetadata,
		InputIDs:            l.resolveInputs(tx.Inputs()),
	})
}

func (l *Ledger) storeAndProcessTransactionDataFlow() *dataflow.DataFlow[*params] {
	return dataflow.New[*params](
		l.storeTransactionCommand,
		l.processTransactionDataFlow().ChainedCommand,
	)
}

func (l *Ledger) processTransactionDataFlow() *dataflow.DataFlow[*params] {
	return dataflow.New[*params](
		l.checkTransactionAlreadyBooked,
		l.checkTransactionDataFlow().ChainedCommand,
		l.bookTransactionCommand,
		l.notifyConsumersCommand,
	).WithErrorCallback(func(err error, params *params) {
		l.ErrorEvent.Trigger(err)

		// TODO: mark Transaction as invalid and trigger invalid event
	})
}

func (l *Ledger) checkTransactionDataFlow() *dataflow.DataFlow[*params] {
	return dataflow.New[*params](
		l.checkSolidityCommand,
		l.checkOutputsCausallyRelatedCommand,
		l.executeTransactionCommand,
	)
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

func (s *Solidifier) checkTransactionAlreadyBooked(params *params, next dataflow.Next[*params]) error {
	if params.TransactionMetadata.Booked() {
		return nil
	}

	return next(params)
}

func (s *Solidifier) resolveInputs(inputs []Input) (outputIDs OutputIDs) {
	return NewOutputIDs(generics.Map(inputs, s.vm.ResolveInput)...)
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
