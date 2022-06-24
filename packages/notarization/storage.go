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
	epochContentStorages map[epoch.Index]*epochContentStorages

	// epochCommitmentStorageOptions is a dictionary for configuration parameters of the Storage.
	epochCommitmentStorageOptions *options

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

type epochContentStorages struct {
	spentOutputs   *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]
	createdOutputs *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]
	MessageIDs     kvstore.KVStore
	TransactionIDs kvstore.KVStore
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

	new.epochContentStorages = make(map[epoch.Index]*epochContentStorages)

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

func (s *EpochCommitmentStorage) SetFullEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("fullEpochIndex", ei)
}

func (s *EpochCommitmentStorage) FullEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("fullEpochIndex")
}

func (s *EpochCommitmentStorage) SetDiffEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("diffEpochIndex", ei)
}

func (s *EpochCommitmentStorage) DiffEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("diffEpochIndex")
}

func (s *EpochCommitmentStorage) SetLastCommittedEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("lastCommittedEpochIndex", ei)
}

func (s *EpochCommitmentStorage) LastCommittedEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("lastCommittedEpochIndex")
}

func (s *EpochCommitmentStorage) SetLastConfirmedEpochIndex(ei epoch.Index) error {
	return s.setIndexFlag("lastConfirmedEpochIndex", ei)
}

func (s *EpochCommitmentStorage) LastConfirmedEpochIndex() (ei epoch.Index, err error) {
	return s.getIndexFlag("lastConfirmedEpochIndex")
}

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.ledgerstateStorage.Shutdown()
		s.ecRecordStorage.Shutdown()
		for _, epochDiffStorage := range s.epochContentStorages {
			epochDiffStorage.spentOutputs.Shutdown()
			epochDiffStorage.createdOutputs.Shutdown()
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

func (s *EpochCommitmentStorage) dropEpochContentStorage(ei epoch.Index) {
	// TODO: properly drop (delete epoch bucketed) storage
	contentStorage := s.getEpochContentStorage(ei)
	contentStorage.spentOutputs.Shutdown()
	contentStorage.createdOutputs.Shutdown()
	_ = contentStorage.MessageIDs.Flush()
	_ = contentStorage.TransactionIDs.Flush()
	delete(s.epochContentStorages, ei)
}

func (s *EpochCommitmentStorage) getEpochContentStorage(ei epoch.Index) (contentStorage *epochContentStorages) {
	if epochContentStorage, exists := s.epochContentStorages[ei]; exists {
		return epochContentStorage
	}

	spentDiffStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, PrefixEpochDiffSpent}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}
	createdDiffStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, PrefixEpochDiffCreated}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}
	messageIDsStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, PrefixEpochMessageIDs}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}

	transactionIDsStore, err := s.baseStore.WithRealm(append([]byte{database.PrefixNotarization, PrefixEpochTransactionIDs}, ei.Bytes()...))
	if err != nil {
		panic(err)
	}

	contentStorage = &epochContentStorages{
		spentOutputs: objectstorage.NewStructStorage[ledger.OutputWithMetadata](
			spentDiffStore,
			s.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(s.epochCommitmentStorageOptions.epochCommitmentCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),

		createdOutputs: objectstorage.NewStructStorage[ledger.OutputWithMetadata](
			createdDiffStore,
			s.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(s.epochCommitmentStorageOptions.epochCommitmentCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		MessageIDs:     messageIDsStore,
		TransactionIDs: transactionIDsStore,
	}

	s.epochContentStorages[ei] = contentStorage

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	PrefixLedgerState byte = iota

	PrefixECRecord

	PrefixEpochDiffCreated

	PrefixEpochDiffSpent

	PrefixEpochMessageIDs

	PrefixEpochTransactionIDs

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
