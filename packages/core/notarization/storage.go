package notarization

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// EpochCommitmentStorage is a Ledger component that bundles the storage related API.
type EpochCommitmentStorage struct {
	// Base store for all other storages, prefixed by the package
	baseStore kvstore.KVStore

	ledgerstateStorage *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]

	ecRecordStorage *objectstorage.ObjectStorage[*epoch.ECRecord]

	// Delta storages
	epochDiffStoragesMutex sync.Mutex
	epochDiffStorages      *shrinkingmap.ShrinkingMap[epoch.Index, *epochDiffStorage]

	// epochCommitmentStorageOptions is a dictionary for configuration parameters of the Storage.
	epochCommitmentStorageOptions *options

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

type epochDiffStorage struct {
	spent   *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]
	created *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]
}

// newEpochCommitmentStorage returns a new storage instance for the given Ledger.
func newEpochCommitmentStorage(options ...Option) (new *EpochCommitmentStorage) {
	new = &EpochCommitmentStorage{
		epochCommitmentStorageOptions: newOptions(options...),
	}

	new.baseStore = new.epochCommitmentStorageOptions.store

	new.ledgerstateStorage = objectstorage.NewStructStorage[ledger.OutputWithMetadata](
		objectstorage.NewStoreWithRealm(new.baseStore, database.PrefixNotarization, prefixLedgerState),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	new.ecRecordStorage = objectstorage.NewStructStorage[epoch.ECRecord](
		objectstorage.NewStoreWithRealm(new.baseStore, database.PrefixNotarization, prefixECRecord),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	new.epochDiffStorages = shrinkingmap.New[epoch.Index, *epochDiffStorage]()

	return new
}

// CachedECRecord retrieves cached ECRecord of the given EI. (Make sure to Release or Consume the return object.)
func (s *EpochCommitmentStorage) CachedECRecord(ei epoch.Index, computeIfAbsentCallback ...func(ei epoch.Index) *epoch.ECRecord) (cachedEpochDiff *objectstorage.CachedObject[*epoch.ECRecord]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.ecRecordStorage.ComputeIfAbsent(ei.Bytes(), func(key []byte) *epoch.ECRecord {
			return computeIfAbsentCallback[0](ei)
		})
	}

	return s.ecRecordStorage.Load(ei.Bytes())
}

// shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) shutdown() {
	s.shutdownOnce.Do(func() {
		s.ledgerstateStorage.Shutdown()
		s.ecRecordStorage.Shutdown()
		s.epochDiffStoragesMutex.Lock()
		defer s.epochDiffStoragesMutex.Unlock()
		s.epochDiffStorages.ForEach(func(_ epoch.Index, epochDiffStorage *epochDiffStorage) bool {
			epochDiffStorage.spent.Shutdown()
			epochDiffStorage.created.Shutdown()
			return true
		})
	})
}

func (s *EpochCommitmentStorage) setLatestCommittableEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("latestCommittableEpochIndex", ei)
}

func (s *EpochCommitmentStorage) latestCommittableEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("latestCommittableEpochIndex")
}

func (s *EpochCommitmentStorage) setLastConfirmedEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("lastConfirmedEpochIndex", ei)
}

func (s *EpochCommitmentStorage) lastConfirmedEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("lastConfirmedEpochIndex")
}

func (s *EpochCommitmentStorage) setAcceptanceEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("acceptanceEpochIndex", ei)
}

func (s *EpochCommitmentStorage) acceptanceEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("acceptanceEpochIndex")
}

func (s *EpochCommitmentStorage) getIndexFlag(flag string) (ei epoch.Index, err error) {
	var value []byte
	if value, err = s.baseStore.Get([]byte(flag)); err != nil {
		return ei, errors.Wrapf(err, "failed to get %s from database", flag)
	}

	if ei, _, err = epoch.IndexFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (s *EpochCommitmentStorage) setIndexFlag(flag string, ei epoch.Index) (err error) {
	if err := s.baseStore.Set([]byte(flag), ei.Bytes()); err != nil {
		return errors.Wrapf(err, "failed to set %s in database", flag)
	}
	return nil
}

func (s *EpochCommitmentStorage) dropEpochDiffStorage(ei epoch.Index) {
	// TODO: properly drop (delete epoch bucketed) storage
	diffStorage := s.getEpochDiffStorage(ei)

	s.epochDiffStoragesMutex.Lock()
	defer s.epochDiffStoragesMutex.Unlock()

	s.epochDiffStorages.Delete(ei)
	go func() {
		diffStorage.spent.Shutdown()
		diffStorage.created.Shutdown()
	}()
}

func (s *EpochCommitmentStorage) getEpochDiffStorage(ei epoch.Index) (diffStorage *epochDiffStorage) {
	s.epochDiffStoragesMutex.Lock()
	defer s.epochDiffStoragesMutex.Unlock()

	if epochDiffStorage, exists := s.epochDiffStorages.Get(ei); exists {
		return epochDiffStorage
	}

	spentDiffStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, prefixEpochDiffSpent}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}
	createdDiffStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, prefixEpochDiffCreated}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}

	diffStorage = &epochDiffStorage{
		spent: objectstorage.NewStructStorage[ledger.OutputWithMetadata](
			spentDiffStore,
			s.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(s.epochCommitmentStorageOptions.epochCommitmentCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),

		created: objectstorage.NewStructStorage[ledger.OutputWithMetadata](
			createdDiffStore,
			s.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(s.epochCommitmentStorageOptions.epochCommitmentCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
	}

	s.epochDiffStorages.Set(ei, diffStorage)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	prefixLedgerState byte = iota

	prefixECRecord

	prefixEpochDiffCreated

	prefixEpochDiffSpent

	prefixStateTreeNodes

	prefixStateTreeValues

	prefixManaTreeNodes

	prefixManaTreeValues
)

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithStore is an Option for the Ledger that allows to configure which KVStore is supposed to be used to persist data
// (the default option is to use a MapDB).
func WithStore(store kvstore.KVStore) (option Option) {
	return func(options *options) {
		options.store = store
	}
}

// options is a container for all configurable parameters of the Indexer.
type options struct {
	// store contains the KVStore that is used to persist data.
	store kvstore.KVStore

	// cacheTimeProvider contains the cacheTimeProvider that overrides the local cache times.
	cacheTimeProvider *database.CacheTimeProvider

	epochCommitmentCacheTime time.Duration
}

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
