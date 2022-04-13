package indexer

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region options //////////////////////////////////////////////////////////////////////////////////////////////////////

// options is a container for all configurable parameters of the Indexer.
type options struct {
	// store contains the KVStore that is used to persist data.
	store kvstore.KVStore

	// cacheTimeProvider contains the cacheTimeProvider that overrides the local cache times.
	cacheTimeProvider *database.CacheTimeProvider

	// transactionCacheTime contains the duration that Transaction objects stay cached after they have been released.
	transactionCacheTime time.Duration
}

// newOptions returns a new options object that corresponds to the handed in options and which is derived from the
// default options.
func newOptions(option ...Option) (new *options) {
	return (&options{
		store:                mapdb.NewMapDB(),
		cacheTimeProvider:    database.NewCacheTimeProvider(0),
		transactionCacheTime: 10 * time.Second,
	}).apply(option...)
}

// apply modifies the options object by overriding the handed in options.
func (o *options) apply(options ...Option) (self *options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// Option represents the return type of optional parameters that can be handed into the constructor of the Ledger
// to configure its behavior.
type Option func(*options)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
