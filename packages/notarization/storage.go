package notarization

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"
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

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

type OutputID struct {
	model.Storable[utxo.OutputID, utxo.OutputID] `serix:"0"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type smtStore struct {
	Nodes  kvstore.KVStore
	Values kvstore.KVStore
}

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// EpochCommitmentStorage is a Ledger component that bundles the storage related API.
type EpochCommitmentStorage struct {
	// Base store for all other storages, prefixed by the package
	baseStore kvstore.KVStore

	// TODO
	smtStores map[EI]*smtStore

	ledgerstateStore *objectstorage.ObjectStorage[utxo.Output]

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

	new.baseStore = new.specializeStore(new.epochCommitmentStorageOptions.store, database.PrefixNotarization)

	ledgerStore := new.specializeStore(new.epochCommitmentStorageOptions.store, PrefixLedgerState)

	new.ledgerstateStore = objectstorage.NewInterfaceStorage[utxo.Output](
		ledgerStore,
		ledger.OutputFactory(new.epochCommitmentStorageOptions.vm),
		new.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(new.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	return new
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) Shutdown() {
	s.shutdownOnce.Do(func() {
		//s.epochStateDiffStore.Shutdown()
	})
}

func (s *EpochCommitmentStorage) specializeStore(baseStore kvstore.KVStore, prefixes ...byte) (specializedStore kvstore.KVStore) {
	specializedStore, err := baseStore.WithRealm(prefixes)
	if err != nil {
		panic(fmt.Errorf("could not create specialized store: %w", err))
	}
	return specializedStore
}

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) getDeltaStore(ei EI) *objectstorage.ObjectStorage[*EpochStateDiff] {
	if store, exists := s.deltaStores[ei]; exists {
		return store
	}

	s.deltaStores[ei] = objectstorage.NewStructStorage[EpochStateDiff](
		objectstorage.NewStoreWithRealm(s.epochCommitmentStorageOptions.store, database.PrefixNotarization, PrefixEpochStateDiff),
		s.epochCommitmentStorageOptions.cacheTimeProvider.CacheTime(s.epochCommitmentStorageOptions.epochCommitmentCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	)

	return s.deltaStores[ei]
}

// Shutdown shuts down the KVStore used to persist data.
func (s *EpochCommitmentStorage) getSmtStore(ei EI) *smtStore {
	if store, exists := s.smtStores[ei]; exists {
		return store
	}

	eiBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(eiBytes, uint64(ei))

	epochSmtStore := s.specializeStore(s.specializeStore(s.baseStore, PrefixSmt), eiBytes...)

	s.smtStores[ei] = &smtStore{
		Nodes: s.specializeStore(epochSmtStore, PrefixSmtNodes),
		Values: s.specializeStore(epochSmtStore, PrefixSmtValues),
	}

	return s.smtStores[ei]
}

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixEpochStateDiff defines the storage prefix for the Transaction object storage.
	PrefixEpochStateDiff byte = iota

	PrefixTangleLeaf

	PrefixMutationLeaf

	PrefixLedgerState

	PrefixSmt

	PrefixSmtNodes

	PrefixSmtValues
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
