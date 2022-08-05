package ledger

import (
	"time"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"

	"github.com/iotaledger/goshimmer/packages/core/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region WithVM ///////////////////////////////////////////////////////////////////////////////////////////////////////

// WithVM is an Option for the Ledger that allows to configure which VM is supposed to be used to process transactions.
func WithVM(vm vm.VM) (option Option) {
	return func(options *options) {
		options.vm = vm
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithStore ////////////////////////////////////////////////////////////////////////////////////////////////////

// WithStore is an Option for the Ledger that allows to configure which KVStore is supposed to be used to persist data
// (the default option is to use a MapDB).
func WithStore(store kvstore.KVStore) (option Option) {
	return func(options *options) {
		options.store = store
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithCacheTimeProvider ////////////////////////////////////////////////////////////////////////////////////////

// WithCacheTimeProvider is an Option for the Ledger that allows to configure which CacheTimeProvider is supposed to
// be used.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) (option Option) {
	return func(options *options) {
		options.cacheTimeProvider = cacheTimeProvider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithTransactionCacheTime /////////////////////////////////////////////////////////////////////////////////////

// WithTransactionCacheTime is an Option for the Ledger that allows to configure how long Transaction objects stay
// cached after they have been released.
func WithTransactionCacheTime(transactionCacheTime time.Duration) (option Option) {
	return func(options *options) {
		options.transactionCacheTime = transactionCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithTransactionMetadataCacheTime /////////////////////////////////////////////////////////////////////////////

// WithTransactionMetadataCacheTime is an Option for the Ledger that allows to configure how long TransactionMetadata
// objects stay cached after they have been released.
func WithTransactionMetadataCacheTime(transactionMetadataCacheTime time.Duration) (option Option) {
	return func(options *options) {
		options.transactionMetadataCacheTime = transactionMetadataCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithOutputCacheTime //////////////////////////////////////////////////////////////////////////////////////////

// WithOutputCacheTime is an Option for the Ledger that allows to configure how long Output objects stay cached after
// they have been released.
func WithOutputCacheTime(outputCacheTime time.Duration) (option Option) {
	return func(options *options) {
		options.outputCacheTime = outputCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithOutputMetadataCacheTime //////////////////////////////////////////////////////////////////////////////////

// WithOutputMetadataCacheTime is an Option for the Ledger that allows to configure how long OutputMetadata objects stay
// cached after they have been released.
func WithOutputMetadataCacheTime(outputMetadataCacheTime time.Duration) (option Option) {
	return func(options *options) {
		options.outputMetadataCacheTime = outputMetadataCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithConsumerCacheTime ////////////////////////////////////////////////////////////////////////////////////////

// WithConsumerCacheTime is an Option for the Ledger that allows to configure how long Consumer objects stay cached
// after they have been released.
func WithConsumerCacheTime(consumerCacheTime time.Duration) (option Option) {
	return func(options *options) {
		options.consumerCacheTime = consumerCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithConsumerCacheTime ////////////////////////////////////////////////////////////////////////////////////////

// WithConflictDAGOptions is an Option for the Ledger that allows to configure the options for the ConflictDAG
func WithConflictDAGOptions(conflictDAGOptions ...conflictdag.Option) (option Option) {
	return func(options *options) {
		options.conflictDAGOptions = conflictDAGOptions
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region options //////////////////////////////////////////////////////////////////////////////////////////////////////

// options is a container for all configurable parameters of the Ledger.
type options struct {
	// vm contains the virtual machine that is used to execute Transactions.
	vm vm.VM

	// store contains the KVStore that is used to persist data.
	store kvstore.KVStore

	// cacheTimeProvider contains the cacheTimeProvider that overrides the local cache times.
	cacheTimeProvider *database.CacheTimeProvider

	// transactionCacheTime contains the duration that Transaction objects stay cached after they have been released.
	transactionCacheTime time.Duration

	// transactionCacheTime contains the duration that TransactionMetadata objects stay cached after they have been
	// released.
	transactionMetadataCacheTime time.Duration

	// outputCacheTime contains the duration that Output objects stay cached after they have been released.
	outputCacheTime time.Duration

	// outputMetadataCacheTime contains the duration that OutputMetadata objects stay cached after they have been
	// released.
	outputMetadataCacheTime time.Duration

	// consumerCacheTime contains the duration that Consumer objects stay cached after they have been released.
	consumerCacheTime time.Duration

	// conflictDAGOptions contains the options for the ConflictDAG.
	conflictDAGOptions []conflictdag.Option
}

// newOptions returns a new options object that corresponds to the handed in options and which is derived from the
// default options.
func newOptions(option ...Option) (new *options) {
	return (&options{
		store:                        mapdb.NewMapDB(),
		cacheTimeProvider:            database.NewCacheTimeProvider(0),
		vm:                           NewMockedVM(),
		transactionCacheTime:         10 * time.Second,
		transactionMetadataCacheTime: 10 * time.Second,
		outputCacheTime:              10 * time.Second,
		outputMetadataCacheTime:      10 * time.Second,
		consumerCacheTime:            10 * time.Second,
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
