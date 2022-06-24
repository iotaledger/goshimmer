package notarization

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// EpochCommitmentStorage is a Ledger component that bundles the storage related API.
type EpochCommitmentStorage struct {
	// Base store for all other storages, prefixed by the package
	baseStore kvstore.KVStore

	ledgerstateStorage *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]

	ecRecordStorage *objectstorage.ObjectStorage[*epoch.ECRecord]

	// Delta storages
	epochDiffStorages map[epoch.Index]*EpochDiffStorage

	// epochCommitmentStorageOptions is a dictionary for configuration parameters of the Storage.
	epochCommitmentStorageOptions *options

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

type EpochDiffStorage struct {
	Spent   *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]
	Created *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]
}

// newEpochCommitmentStorage returns a new storage instance for the given Ledger.
func newEpochCommitmentStorage(options ...Option) (new *EpochCommitmentStorage) {
	new = &EpochCommitmentStorage{
		epochCommitmentStorageOptions: newOptions(options...),
	}

	new.baseStore = new.epochCommitmentStorageOptions.store

	new.ledgerstateStorage = objectstorage.NewStructStorage[ledger.OutputWithMetadata](
		objectstorage.NewStoreWithRealm(new.baseStore, database.PrefixNotarization, PrefixLedgerState),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	new.ecRecordStorage = objectstorage.NewStructStorage[epoch.ECRecord](
		objectstorage.NewStoreWithRealm(new.baseStore, database.PrefixNotarization, PrefixECRecord),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	new.epochDiffStorages = make(map[epoch.Index]*EpochDiffStorage)

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

func (s *EpochCommitmentStorage) SetLastCommittedEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("lastCommittedEpochIndex", ei)
}

func (s *EpochCommitmentStorage) LastCommittedEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("lastCommittedEpochIndex")
}

func (s *EpochCommitmentStorage) SetLatestCommittableEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("latestCommittableEpochIndex", ei)
}

func (s *EpochCommitmentStorage) LatestCommittableEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("latestCommittableEpochIndex")
}

func (s *EpochCommitmentStorage) SetLastConfirmedEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("lastConfirmedEpochIndex", ei)
}

func (s *EpochCommitmentStorage) LastConfirmedEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("lastConfirmedEpochIndex")
}

func (s *EpochCommitmentStorage) SetCurrentEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("currentEpochIndex", ei)
}

func (s *EpochCommitmentStorage) CurrentEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("currentEpochIndex")
}

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.ledgerstateStorage.Shutdown()
		s.ecRecordStorage.Shutdown()
		for _, epochDiffStorage := range s.epochDiffStorages {
			epochDiffStorage.Spent.Shutdown()
			epochDiffStorage.Created.Shutdown()
		}
	})
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
	diffStorage.Spent.Shutdown()
	diffStorage.Created.Shutdown()
	delete(s.epochDiffStorages, ei)
}

func (s *EpochCommitmentStorage) getEpochDiffStorage(ei epoch.Index) (diffStorage *EpochDiffStorage) {
	if epochDiffStorage, exists := s.epochDiffStorages[ei]; exists {
		return epochDiffStorage
	}

	spentDiffStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, PrefixEpochDiffSpent}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}
	createdDiffStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, PrefixEpochDiffCreated}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}

	diffStorage = &EpochDiffStorage{
		Spent: objectstorage.NewStructStorage[ledger.OutputWithMetadata](
			spentDiffStore,
			s.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(s.epochCommitmentStorageOptions.epochCommitmentCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),

		Created: objectstorage.NewStructStorage[ledger.OutputWithMetadata](
			createdDiffStore,
			s.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(s.epochCommitmentStorageOptions.epochCommitmentCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
	}

	s.epochDiffStorages[ei] = diffStorage

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	PrefixLedgerState byte = iota

	PrefixECRecord

	PrefixEpochDiffCreated

	PrefixEpochDiffSpent

	PrefixStateTreeNodes

	PrefixStateTreeValues

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
