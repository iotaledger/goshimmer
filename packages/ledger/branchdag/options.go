package branchdag

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region WithStore ////////////////////////////////////////////////////////////////////////////////////////////////////

// WithStore is an Option for the BranchDAG that allows to configure which KVStore is supposed to be used to persist
// data (the default option is to use a MapDB).
func WithStore(store kvstore.KVStore) Option {
	return func(options *options) {
		options.store = store
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithCacheTimeProvider ////////////////////////////////////////////////////////////////////////////////////////

// WithCacheTimeProvider is an Option for the BranchDAG that allows to configure which CacheTimeProvider is supposed to
// be used (the default option is to use a forced CacheTime of 0).
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *options) {
		options.cacheTimeProvider = cacheTimeProvider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Option ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the BranchDAG to
// configure its behavior.
type Option func(*options)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region options //////////////////////////////////////////////////////////////////////////////////////////////////////

// options is a container for all configurable parameters of a BranchDAG.
type options struct {
	store             kvstore.KVStore
	cacheTimeProvider *database.CacheTimeProvider
}

// newOptions returns a new options object that corresponds to the hand in options and is derived from the default
// options.
func newOptions(option ...Option) (new *options) {
	return (&options{
		store:             mapdb.NewMapDB(),
		cacheTimeProvider: database.NewCacheTimeProvider(0),
	}).apply(option...)
}

// apply modifies the options object by overriding the given options.
func (o *options) apply(options ...Option) (self *options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
