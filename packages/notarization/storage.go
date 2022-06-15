package notarization

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/serix"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// EpochCommitmentStorage is a Ledger component that bundles the storage related API.
type EpochCommitmentStorage struct {
	// Base store for all other storages, prefixed by the package
	baseStore kvstore.KVStore

	ledgerstateStorage *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]

	ecRecordStorage *objectstorage.ObjectStorage[*epoch.ECRecord]

	// Delta storages
	epochDiffStorages map[epoch.Index]*epochDiffStorage

	// epochCommitmentStorageOptions is a dictionary for configuration parameters of the Storage.
	epochCommitmentStorageOptions *options

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

type epochDiffStorage struct {
	spent   *objectstorage.ObjectStorage[utxo.Output]
	created *objectstorage.ObjectStorage[*ledger.OutputWithMetadata]
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

	new.epochDiffStorages = make(map[epoch.Index]*epochDiffStorage)

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
	if err := s.baseStore.Set([]byte("fullEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set fullEpochIndex in database")
	}
	return nil
}

func (s *EpochCommitmentStorage) FullEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = s.baseStore.Get([]byte("fullEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get fullEpochIndex from database")
	}

	if ei, _, err = epoch.IndexFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (s *EpochCommitmentStorage) SetDiffEpochIndex(ei epoch.Index) error {
	if err := s.baseStore.Set([]byte("diffEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set diffEpochIndex in database")
	}
	return nil
}

func (s *EpochCommitmentStorage) DiffEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = s.baseStore.Get([]byte("diffEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get diffEpochIndex from database")
	}

	if ei, _, err = epoch.IndexFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (s *EpochCommitmentStorage) SetLastCommittedEpochIndex(ei epoch.Index) error {
	if err := s.baseStore.Set([]byte("lastCommittedEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set lastCommittedEpochIndex in database")
	}
	return nil
}

func (s *EpochCommitmentStorage) LastCommittedEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = s.baseStore.Get([]byte("lastCommittedEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get lastCommittedEpochIndex from database")
	}

	if ei, _, err = epoch.IndexFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

func (s *EpochCommitmentStorage) SetLastConfirmedEpochIndex(ei epoch.Index) error {
	if err := s.baseStore.Set([]byte("lastConfirmedEpochIndex"), ei.Bytes()); err != nil {
		return errors.Wrap(err, "failed to set lastConfirmedEpochIndex in database")
	}
	return nil
}

func (s *EpochCommitmentStorage) LastConfirmedEpochIndex() (ei epoch.Index, err error) {
	var value []byte
	if value, err = s.baseStore.Get([]byte("lastConfirmedEpochIndex")); err != nil {
		return ei, errors.Wrap(err, "failed to get lastConfirmedEpochIndex from database")
	}

	if ei, _, err = epoch.IndexFromBytes(value); err != nil {
		return ei, errors.Wrap(err, "failed to deserialize EI from bytes")
	}

	return
}

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.ledgerstateStorage.Shutdown()
		s.ecRecordStorage.Shutdown()
		for _, epochDiffStorage := range s.epochDiffStorages {
			epochDiffStorage.spent.Shutdown()
			epochDiffStorage.created.Shutdown()
		}
	})
}

func (s *EpochCommitmentStorage) getEpochDiffStorage(ei epoch.Index) (diffStorage *epochDiffStorage) {
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

	diffStorage = &epochDiffStorage{
		spent: objectstorage.NewInterfaceStorage[utxo.Output](
			spentDiffStore,
			outputFactory,
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

	s.epochDiffStorages[ei] = diffStorage

	return
}

// commitLedgerState commits the corresponding diff to the ledger state and drops it.
func (s *EpochCommitmentStorage) commitLedgerState(ei epoch.Index) (err error) {
	fmt.Println("\t\t>> commitLedgerState", ei)
	epochDiffStorage := s.getEpochDiffStorage(ei)
	epochDiffStorage.spent.ForEach(func(_ []byte, cachedOutput *objectstorage.CachedObject[utxo.Output]) bool {
		spentOutput := cachedOutput.Get()
		s.ledgerstateStorage.Delete(spentOutput.ID().Bytes())

		return true
	})
	fmt.Println("\t\t>> commitLedgerState: loaded spent outputs")
	epochDiffStorage.created.ForEach(func(_ []byte, cachedOutputWithMetadata *objectstorage.CachedObject[*ledger.OutputWithMetadata]) bool {
		outputWithMetadata := cachedOutputWithMetadata.Get()
		s.ledgerstateStorage.Store(outputWithMetadata)

		return true
	})
	fmt.Println("\t\t>> commitLedgerState: loaded created outputs")

	delete(s.epochDiffStorages, ei)

	return nil
}

func outputFactory(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	var outputID utxo.OutputID
	if _, err = serix.DefaultAPI.Decode(context.Background(), key, &outputID, serix.WithValidation()); err != nil {
		return nil, err
	}

	output, err := devnetvm.OutputFromBytes(data)
	if err != nil {
		return nil, err
	}
	output.SetID(outputID)

	return output, nil
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
