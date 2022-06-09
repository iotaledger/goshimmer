package notarization

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// EpochCommitmentStorage is a Ledger component that bundles the storage related API.
type EpochCommitmentStorage struct {
	// Base store for all other storages, prefixed by the package
	baseStore kvstore.KVStore

	ledgerstateStorage *objectstorage.ObjectStorage[utxo.Output]

	ecRecordStorage *objectstorage.ObjectStorage[*ECRecord]

	// Delta storages
	epochDiffStorage *objectstorage.ObjectStorage[*epoch.EpochDiff]

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

	new.baseStore = specializeStore(new.epochCommitmentStorageOptions.store, database.PrefixNotarization)

	new.ledgerstateStorage = objectstorage.NewInterfaceStorage[utxo.Output](
		specializeStore(new.baseStore, PrefixLedgerState),
		ledger.OutputFactory(new.epochCommitmentStorageOptions.vm),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)
	new.ecRecordStorage = objectstorage.NewStructStorage[ECRecord](
		specializeStore(new.baseStore, PrefixEC),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	new.epochDiffStorage = objectstorage.NewStructStorage[epoch.EpochDiff](
		specializeStore(new.baseStore, PrefixEpochDiff),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	return new
}

func (s *EpochCommitmentStorage) CachedDiff(ei epoch.EI, computeIfAbsentCallback ...func(ei epoch.EI) *epoch.EpochDiff) (cachedEpochDiff *objectstorage.CachedObject[*epoch.EpochDiff]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.epochDiffStorage.ComputeIfAbsent(ei.Bytes(), func(key []byte) *epoch.EpochDiff {
			return computeIfAbsentCallback[0](ei)
		})
	}

	return s.epochDiffStorage.Load(ei.Bytes())
}

func (s *EpochCommitmentStorage) CachedECRecord(ei epoch.EI, computeIfAbsentCallback ...func(ei epoch.EI) *ECRecord) (cachedEpochDiff *objectstorage.CachedObject[*ECRecord]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.ecRecordStorage.ComputeIfAbsent(ei.Bytes(), func(key []byte) *ECRecord {
			return computeIfAbsentCallback[0](ei)
		})
	}

	return s.ecRecordStorage.Load(ei.Bytes())
}

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.ledgerstateStorage.Shutdown()
		s.ecRecordStorage.Shutdown()
		s.epochDiffStorage.Shutdown()
	})
}

func specializeStore(baseStore kvstore.KVStore, prefixes ...byte) (specializedStore kvstore.KVStore) {
	specializedStore, err := baseStore.WithRealm(prefixes)
	if err != nil {
		panic(fmt.Errorf("could not create specialized store: %w", err))
	}
	return specializedStore
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixEpochStateDiff defines the storage prefix for the Transaction object storage.
	PrefixEpochStateDiff byte = iota

	PrefixLedgerState

	PrefixEC

	PrefixEpochDiff

	PrefixStateTree

	PrefixStateTreeNodes

	PrefixStateTreeValues

	PrefixManaTree

	PrefixManaTreeNodes

	PrefixManaTreeValues
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

// region WithVM ////////////////////////////////////////////////////////////////////////////////////////////////////

// TODO
func WithVM(vm vm.VM) (option Option) {
	return func(options *options) {
		options.vm = vm
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

	//
	vm vm.VM
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
