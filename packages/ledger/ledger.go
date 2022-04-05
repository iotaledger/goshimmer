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
	BranchDAG *branchdag.BranchDAG
	Storage   *storage

	validator *validator
	booker    *booker
	dataFlow  *dataFlow
	utils     *utils
	options   *options
	mutex     *syncutils.DAGMutex[utxo.TransactionID]
}

func New(options ...Option) (ledger *Ledger) {
	ledger = &Ledger{
		Events:  newEvents(),
		options: newOptions(options...),
		mutex:   syncutils.NewDAGMutex[utxo.TransactionID](),
	}

	ledger.BranchDAG = branchdag.New(branchdag.WithStore(ledger.options.Store), branchdag.WithCacheTimeProvider(ledger.options.CacheTimeProvider))
	ledger.Storage = newStorage(ledger)
	ledger.validator = newValidator(ledger)
	ledger.booker = newBooker(ledger)
	ledger.dataFlow = newDataFlow(ledger)
	ledger.utils = newUtils(ledger)

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

// region options //////////////////////////////////////////////////////////////////////////////////////////////////////

// options is a container for all configurable parameters of the Ledger.
type options struct {
	Store             kvstore.KVStore
	CacheTimeProvider *database.CacheTimeProvider
	VM                vm.VM
}

func newOptions(option ...Option) (new *options) {
	return (&options{
		Store:             mapdb.NewMapDB(),
		CacheTimeProvider: database.NewCacheTimeProvider(0),
		VM:                NewMockedVM(),
	}).Apply(option...)
}

func (o *options) Apply(options ...Option) (self *options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// Option represents the return type of optional parameters that can be handed into the constructor of the Ledger
// to configure its behavior.
type Option func(*options)

// WithStore is an Option for the Ledger that allows to specify which storage layer is supposed to be used to persist
// data.
func WithStore(store kvstore.KVStore) Option {
	return func(options *options) {
		options.Store = store
	}
}

// WithCacheTimeProvider is an Option for the Tangle that allows to override hard coded cache time.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *options) {
		options.CacheTimeProvider = cacheTimeProvider
	}
}

func WithVM(vm vm.VM) Option {
	return func(options *options) {
		options.VM = vm
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
