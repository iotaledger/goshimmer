package ledger

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/syncutils"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	TransactionStoredEvent *event.Event[utxo.TransactionID]
	TransactionBookedEvent *event.Event[utxo.TransactionID]
	ErrorEvent             *event.Event[error]

	*Storage
	*Validator
	*Booker

	*DataFlow
	*Options
	*Utils

	*branchdag.BranchDAG
	*syncutils.DAGMutex[[32]byte]
}

func New(store kvstore.KVStore, vm utxo.VM, options ...Option) (ledger *Ledger) {
	ledger = &Ledger{
		TransactionStoredEvent: event.New[utxo.TransactionID](),
		TransactionBookedEvent: event.New[utxo.TransactionID](),
		ErrorEvent:             event.New[error](),

		BranchDAG: branchdag.NewBranchDAG(store, database.NewCacheTimeProvider(0)),
		DAGMutex:  syncutils.NewDAGMutex[[32]byte](),
	}

	ledger.Configure(options...)

	ledger.DataFlow = NewDataFlow(ledger)
	ledger.Storage = NewStorage(ledger)
	ledger.Validator = NewValidator(ledger, vm)
	ledger.Booker = NewBooker(ledger)
	ledger.Utils = NewUtils(ledger)

	return ledger
}

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

func (l *Ledger) StoreAndProcessTransaction(tx utxo.Transaction) (err error) {
	l.Lock(tx.ID())
	defer l.Unlock(tx.ID())

	return l.DataFlow.storeAndProcessTransaction().Run(&dataFlowParams{Transaction: NewTransaction(tx)})
}

func (l *Ledger) CheckTransaction(tx utxo.Transaction) (err error) {
	return l.DataFlow.checkTransaction().Run(&dataFlowParams{Transaction: NewTransaction(tx)})
}

func (l *Ledger) processTransaction(tx *Transaction) (err error) {
	l.Lock(tx.ID())
	defer l.Unlock(tx.ID())

	return l.DataFlow.processTransaction().Run(&dataFlowParams{Transaction: tx})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the Ledger
// to configure its behavior.
type Option func(*Options)

// Options is a container for all configurable parameters of the Ledger.
type Options struct {
	Store              kvstore.KVStore
	CacheTimeProvider  *database.CacheTimeProvider
	LazyBookingEnabled bool
}

// Store is an Option for the Ledger that allows to specify which storage layer is supposed to be used to persist
// data.
func Store(store kvstore.KVStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}

// CacheTimeProvider is an Option for the Tangle that allows to override hard coded cache time.
func CacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *Options) {
		options.CacheTimeProvider = cacheTimeProvider
	}
}

// LazyBookingEnabled is an Option for the Ledger that allows to specify if the ledger state should lazy book
// conflicts that look like they have been decided already.
func LazyBookingEnabled(enabled bool) Option {
	return func(options *Options) {
		options.LazyBookingEnabled = enabled
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
