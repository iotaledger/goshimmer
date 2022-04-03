package old

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"
)

// region Ledger //////////////////////////////////////////////////////////////////////////////////////////////////

// Ledger is a data structure that follows the principles of the quadruple entry accounting.
type Ledger struct {
	*Options
	*UTXODAG
	*branchdag.BranchDAG
}

// New is the constructor for the Ledger.
func New(options ...Option) (ledgerstate *Ledger) {
	ledgerstate = &Ledger{}
	ledgerstate.Configure(options...)

	ledgerstate.UTXODAG = NewUTXODAG(ledgerstate)
	ledgerstate.BranchDAG = branchdag.NewBranchDAG(ledgerstate)

	return ledgerstate
}

// Configure modifies the configuration of the Ledger.
func (l *Ledger) Configure(options ...Option) {
	if l.Options == nil {
		l.Options = &Options{
			Store:              mapdb.NewMapDB(),
			LazyBookingEnabled: true,
		}
	}

	for _, option := range options {
		option(l.Options)
	}
}

// Shutdown marks the Ledger as stopped, so it will not accept any new Transactions.
func (l *Ledger) Shutdown() {
	l.BranchDAG.Shutdown()
	l.UTXODAG.Shutdown()
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
