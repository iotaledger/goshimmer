package ledger

import (
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/txvm"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	transactionStorage         *objectstorage.ObjectStorage[*Transaction]
	transactionMetadataStorage *objectstorage.ObjectStorage[*TransactionMetadata]
	outputStorage              *objectstorage.ObjectStorage[*Output]
	outputMetadataStorage      *objectstorage.ObjectStorage[*OutputMetadata]
	consumerStorage            *objectstorage.ObjectStorage[*Consumer]

	*Ledger
}

func NewStorage(ledger *Ledger) (newStorage *Storage) {
	return &Storage{
		transactionStorage: objectstorage.New[*Transaction](
			ledger.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionStorage}),
			ledger.Options.CacheTimeProvider.CacheTime(transactionCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		transactionMetadataStorage: objectstorage.New[*TransactionMetadata](
			ledger.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionMetadataStorage}),
			ledger.Options.CacheTimeProvider.CacheTime(transactionCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		outputStorage: objectstorage.New[*Output](
			ledger.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixOutputStorage}),
			ledger.Options.CacheTimeProvider.CacheTime(outputCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
			objectstorage.WithObjectFactory(txvm.OutputEssenceFromObjectStorage),
		),
		outputMetadataStorage: objectstorage.New[*OutputMetadata](
			ledger.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixOutputMetadataStorage}),
			ledger.Options.CacheTimeProvider.CacheTime(outputCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		consumerStorage: objectstorage.New[*Consumer](
			ledger.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixConsumerStorage}),
			ledger.Options.CacheTimeProvider.CacheTime(consumerCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			ConsumerPartitionKeys,
		),

		Ledger: ledger,
	}
}

// CachedTransaction retrieves the Transaction with the given TransactionID from the object storage.
func (s *Storage) CachedTransaction(transactionID utxo.TransactionID) (cachedTransaction *objectstorage.CachedObject[*Transaction]) {
	return s.transactionStorage.Load(transactionID.Bytes())
}

// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (s *Storage) CachedTransactionMetadata(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) *TransactionMetadata) (cachedTransactionMetadata *objectstorage.CachedObject[*TransactionMetadata]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.transactionMetadataStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) *TransactionMetadata {
			return computeIfAbsentCallback[0](transactionID)
		})
	}

	return s.transactionMetadataStorage.Load(transactionID.Bytes())
}

// CachedOutput retrieves the Output with the given OutputID from the object storage.
func (s *Storage) CachedOutput(outputID utxo.OutputID) (cachedOutput CachedOutput) {
	return s.outputStorage.Load(outputID.Bytes())
}

// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
func (s *Storage) CachedOutputMetadata(outputID utxo.OutputID) (cachedOutput *objectstorage.CachedObject[*OutputMetadata]) {
	return s.outputMetadataStorage.Load(outputID.Bytes())
}

// CachedConsumer retrieves the OutputMetadata with the given OutputID from the object storage.
func (s *Storage) CachedConsumer(outputID utxo.OutputID, txID utxo.TransactionID) (cachedOutput *objectstorage.CachedObject[*Consumer]) {
	return s.consumerStorage.Load(byteutils.ConcatBytes(outputID.Bytes(), txID.Bytes()))
}

// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
func (s *Storage) CachedConsumers(outputID utxo.OutputID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	s.consumerStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Consumer]) bool {
		cachedConsumers = append(cachedConsumers, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(outputID.Bytes()))

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DB prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixTransactionStorage defines the storage prefix for the Transaction object storage.
	PrefixTransactionStorage byte = iota

	// PrefixTransactionMetadataStorage defines the storage prefix for the TransactionMetadata object storage.
	PrefixTransactionMetadataStorage

	// PrefixOutputStorage defines the storage prefix for the OutputEssence object storage.
	PrefixOutputStorage

	// PrefixOutputMetadataStorage defines the storage prefix for the OutputMetadata object storage.
	PrefixOutputMetadataStorage

	// PrefixConsumerStorage defines the storage prefix for the Consumer object storage.
	PrefixConsumerStorage
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and configurations /////////////////////////////////////////////////////////////////////////////////

const (
	transactionCacheTime = 10 * time.Second
	outputCacheTime      = 10 * time.Second
	consumerCacheTime    = 10 * time.Second
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
