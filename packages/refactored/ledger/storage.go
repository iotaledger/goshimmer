package ledger

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/dataflow"
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

func (s *Storage) storeTransactionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	created := false
	cachedTransactionMetadata := s.CachedTransactionMetadata(params.Transaction.ID(), func(txID utxo.TransactionID) *TransactionMetadata {
		s.transactionStorage.Store(params.Transaction).Release()
		created = true
		return NewTransactionMetadata(txID)
	})
	defer cachedTransactionMetadata.Release()

	params.TransactionMetadata, _ = cachedTransactionMetadata.Unwrap()

	if !created {
		if params.TransactionMetadata.Booked() {
			return nil
		}

		return errors.Errorf("%s is an unsolid reattachment: %w", params.Transaction.ID(), ErrTransactionUnsolid)
	}

	params.InputIDs = s.resolveInputs(params.Transaction.Inputs())

	cachedConsumers := s.initConsumers(params.InputIDs, params.Transaction.ID())
	defer cachedConsumers.Release()
	params.Consumers = cachedConsumers.Unwrap(true)

	s.TransactionStoredEvent.Trigger(&TransactionStoredEvent{
		TransactionID: params.Transaction.ID(),
	})

	return next(params)
}

func (s *Storage) initConsumers(outputIDs utxo.OutputIDs, txID utxo.TransactionID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	_ = outputIDs.ForEach(func(outputID utxo.OutputID) (err error) {
		cachedConsumers = append(cachedConsumers, s.CachedConsumer(outputID, txID, NewConsumer))
		return nil
	})

	return cachedConsumers
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
func (s *Storage) CachedOutput(outputID utxo.OutputID) (cachedOutput *objectstorage.CachedObject[*Output]) {
	return s.outputStorage.Load(outputID.Bytes())
}

func (s *Storage) CachedOutputs(outputIDs utxo.OutputIDs) (cachedOutputs objectstorage.CachedObjects[*Output]) {
	cachedOutputs = make(objectstorage.CachedObjects[*Output], 0)
	_ = outputIDs.ForEach(func(outputID utxo.OutputID) error {
		cachedOutputs = append(cachedOutputs, s.CachedOutput(outputID))
		return nil
	})

	return cachedOutputs
}

// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
func (s *Storage) CachedOutputMetadata(outputID utxo.OutputID) (cachedOutputMetadata *objectstorage.CachedObject[*OutputMetadata]) {
	return s.outputMetadataStorage.Load(outputID.Bytes())
}

func (s *Storage) CachedOutputsMetadata(outputIDs utxo.OutputIDs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], 0)
	_ = outputIDs.ForEach(func(outputID utxo.OutputID) error {
		cachedOutputsMetadata = append(cachedOutputsMetadata, s.CachedOutputMetadata(outputID))
		return nil
	})

	return cachedOutputsMetadata
}

// CachedConsumer retrieves the OutputMetadata with the given OutputID from the object storage.
func (s *Storage) CachedConsumer(outputID utxo.OutputID, txID utxo.TransactionID, computeIfAbsentCallback ...func(outputID utxo.OutputID, txID utxo.TransactionID) *Consumer) (cachedOutput *objectstorage.CachedObject[*Consumer]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.consumerStorage.ComputeIfAbsent(byteutils.ConcatBytes(outputID.Bytes(), txID.Bytes()), func(key []byte) *Consumer {
			return computeIfAbsentCallback[0](outputID, txID)
		})
	}

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
