package ledger

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Ledger is an implementation of a "realities-ledger" that manages Outputs according to the principles of the
// quadruple-entry accounting. It acts as a wrapper for the underlying components and exposes the public facing API.
type Ledger struct {
	Events    *Events
	Options   *Options
	BranchDAG *branchdag.BranchDAG
	Storage   *storage

	dataFlow  *dataFlow
	validator *validator
	booker    *booker
	utils     *utils
	mutex     *syncutils.DAGMutex[utxo.TransactionID]
}

func New(options ...Option) (ledger *Ledger) {
	ledger = &Ledger{
		Events:  NewEvents(),
		Options: NewOptions(options...),
		mutex:   syncutils.NewDAGMutex[utxo.TransactionID](),
	}

	ledger.Storage = newStorage(ledger)
	ledger.dataFlow = newDataFlow(ledger)
	ledger.validator = newValidator(ledger)
	ledger.booker = newBooker(ledger)
	ledger.utils = newUtils(ledger)

	ledger.BranchDAG = branchdag.NewBranchDAG(
		branchdag.WithStore(ledger.Options.Store),
		branchdag.WithCacheTimeProvider(ledger.Options.CacheTimeProvider),
	)

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
	for it := l.utils.UnprocessedConsumingTransactions(outputIDs).Iterator(); it.HasNext(); {
		consumingTransactionID := it.Next()
		go l.Storage.CachedTransaction(consumingTransactionID).Consume(func(tx *Transaction) {
			_ = l.processTransaction(tx)
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Options is a container for all configurable parameters of the Ledger.
type Options struct {
	Store             kvstore.KVStore
	CacheTimeProvider *database.CacheTimeProvider
	VM                vm.VM
}

func NewOptions(options ...Option) (new *Options) {
	return (&Options{
		Store:             mapdb.NewMapDB(),
		CacheTimeProvider: database.NewCacheTimeProvider(0),
		VM:                NewMockedVM(),
	}).Apply(options...)
}

func (o *Options) Apply(options ...Option) (self *Options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// Option represents the return type of optional parameters that can be handed into the constructor of the Ledger
// to configure its behavior.
type Option func(*Options)

// WithStore is an Option for the Ledger that allows to specify which storage layer is supposed to be used to persist
// data.
func WithStore(store kvstore.KVStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}

// WithCacheTimeProvider is an Option for the Tangle that allows to override hard coded cache time.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *Options) {
		options.CacheTimeProvider = cacheTimeProvider
	}
}

func WithVM(vm vm.VM) Option {
	return func(options *Options) {
		options.VM = vm
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
