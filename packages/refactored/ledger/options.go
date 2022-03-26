package ledger

import (
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/database"
)

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
