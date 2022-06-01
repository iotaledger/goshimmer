package notarization

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region EpochStateDiff ///////////////////////////////////////////////////////////////////////////////////////////////

type EpochStateDiff struct {
	model.Storable[EI, epochStateDiff] `serix:"0"`
}

type epochStateDiff struct {
	Spent   devnetvm.Outputs `serix:"0"`
	Created devnetvm.Outputs `serix:"1"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TangleLeaf ///////////////////////////////////////////////////////////////////////////////////////////////

type TangleLeaf struct {
	model.Storable[EI, tangle.MessageID] `serix:"0"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TangleLeaf ///////////////////////////////////////////////////////////////////////////////////////////////

type MutationLeaf struct {
	model.Storable[EI, utxo.TransactionID] `serix:"0"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// EpochCommitmentStorage is a Ledger component that bundles the storage related API.
type EpochCommitmentStorage struct {
	// epochStateDiffStore contains the storage for epoch diffs.
	epochStateDiffStore *objectstorage.ObjectStorage[*EpochStateDiff]

	// TODO
	smtStores map[EI]kvstore.KVStore

	ledgerstateStore *objectstorage.ObjectStorage[*utxo.OutputID]

	// Delta storages
	deltaStores map[EI]*objectstorage.ObjectStorage[*EpochStateDiff]

	// epochCommitmentStorageOptions is a dictionary for configuration parameters of the Storage.
	epochCommitmentStorageOptions *options

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

// newEpochCommitmentStorage returns a new storage instance for the given Ledger.
func newEpochCommitmentStorage(options ...Option) (new *EpochCommitmentStorage) {
	new = &EpochCommitmentStorage{
		epochCommitmentStorageOptions: newOptions(options...),
	}

	new.ledgerstateStore = objectstorage.NewStructStorage[utxo.OutputID](
		objectstorage.NewStoreWithRealm(new.epochCommitmentStorageOptions.store, database.PrefixNotarization, PrefixEpochStateDiff),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	new.epochStateDiffStore = objectstorage.NewStructStorage[EpochStateDiff](
		objectstorage.NewStoreWithRealm(new.epochCommitmentStorageOptions.store, database.PrefixNotarization, PrefixLedgerState),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	new.masterStore = objectstorage.NewStoreWithRealm(new.epochCommitmentStorageOptions.store, database.PrefixNotarization, PrefixLedgerState)

	return new
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.epochStateDiffStore.Shutdown()
	})
}

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixEpochStateDiff defines the storage prefix for the Transaction object storage.
	PrefixEpochStateDiff byte = iota

	PrefixTangleLeaf

	PrefixMutationLeaf

	PrefixLedgerState
)

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

// options is a container for all configurable parameters of the Indexer.
type options struct {
	// store contains the KVStore that is used to persist data.
	store kvstore.KVStore

	// cacheTimeProvider contains the cacheTimeProvider that overrides the local cache times.
	cacheTimeProvider *database.CacheTimeProvider

	// TODO
	epochCommitmentCacheTime time.Duration
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// newOptions returns a new options object that corresponds to the handed in options and which is derived from the
// default options.
func newOptions(option ...Option) (new *options) {
	return (&options{
		store:                    mapdb.NewMapDB(),
		cacheTimeProvider:        database.NewCacheTimeProvider(0),
		epochCommitmentCacheTime: 10 * time.Second,
	}).apply(option...)
}

// apply modifies the options object by overriding the handed in options.
func (o *options) apply(options ...Option) (self *options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// Option represents the return type of optional parameters that can be handed into the constructor of the EpochStateDiffStorage
// to configure its behavior.
type Option func(*options)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
