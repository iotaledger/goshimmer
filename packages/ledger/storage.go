package ledger

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type storage struct {
	transactionStorage         *objectstorage.ObjectStorage[*Transaction]
	transactionMetadataStorage *objectstorage.ObjectStorage[*TransactionMetadata]
	outputStorage              *objectstorage.ObjectStorage[*Output]
	outputMetadataStorage      *objectstorage.ObjectStorage[*OutputMetadata]
	consumerStorage            *objectstorage.ObjectStorage[*Consumer]

	ledger *Ledger
}

func transactionFactory(vm vm.VM) func(key []byte, data []byte) (output objectstorage.StorableObject, err error) {
	return func(key []byte, data []byte) (output objectstorage.StorableObject, err error) {
		var txID utxo.TransactionID
		if err = txID.FromMarshalUtil(marshalutil.New(key)); err != nil {
			panic(err)
			return nil, err
		}

		parsedTransaction, err := vm.ParseTransaction(data)
		if err != nil {
			panic(err)
			return nil, err
		}
		parsedTransaction.SetID(txID)

		return NewTransaction(parsedTransaction), nil
	}
}

func outputFactory(vm vm.VM) func(key []byte, data []byte) (output objectstorage.StorableObject, err error) {
	return func(key []byte, data []byte) (output objectstorage.StorableObject, err error) {
		var outputID utxo.OutputID
		if err = outputID.FromMarshalUtil(marshalutil.New(key)); err != nil {
			return nil, err
		}

		parsedOutput, err := vm.ParseOutput(data)
		if err != nil {
			return nil, err
		}
		parsedOutput.SetID(outputID)

		return NewOutput(parsedOutput), nil
	}
}

func newStorage(ledger *Ledger) (newStorage *storage) {
	return &storage{
		transactionStorage: objectstorage.New[*Transaction](
			ledger.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionStorage}),
			ledger.options.CacheTimeProvider.CacheTime(transactionCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
			objectstorage.WithObjectFactory(transactionFactory(ledger.options.VM)),
		),
		transactionMetadataStorage: objectstorage.New[*TransactionMetadata](
			ledger.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionMetadataStorage}),
			ledger.options.CacheTimeProvider.CacheTime(transactionCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		outputStorage: objectstorage.New[*Output](
			ledger.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixOutputStorage}),
			ledger.options.CacheTimeProvider.CacheTime(outputCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
			objectstorage.WithObjectFactory(outputFactory(ledger.options.VM)),
		),
		outputMetadataStorage: objectstorage.New[*OutputMetadata](
			ledger.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixOutputMetadataStorage}),
			ledger.options.CacheTimeProvider.CacheTime(outputCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		consumerStorage: objectstorage.New[*Consumer](
			ledger.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixConsumerStorage}),
			ledger.options.CacheTimeProvider.CacheTime(consumerCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			ConsumerPartitionKeys,
		),

		ledger: ledger,
	}
}

func (s *storage) storeTransactionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
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

	params.InputIDs = s.ledger.utils.resolveInputs(params.Transaction.Inputs())

	cachedConsumers := s.initConsumers(params.InputIDs, params.Transaction.ID())
	defer cachedConsumers.Release()
	params.Consumers = cachedConsumers.Unwrap(true)

	s.ledger.Events.TransactionStored.Trigger(&TransactionStoredEvent{
		TransactionID: params.Transaction.ID(),
	})

	return next(params)
}

func (s *storage) initConsumers(outputIDs utxo.OutputIDs, txID utxo.TransactionID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	_ = outputIDs.ForEach(func(outputID utxo.OutputID) (err error) {
		cachedConsumers = append(cachedConsumers, s.CachedConsumer(outputID, txID, NewConsumer))
		return nil
	})

	return cachedConsumers
}

// CachedTransaction retrieves the Transaction with the given TransactionID from the object storage.
func (s *storage) CachedTransaction(transactionID utxo.TransactionID) (cachedTransaction *objectstorage.CachedObject[*Transaction]) {
	return s.transactionStorage.Load(transactionID.Bytes())
}

// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (s *storage) CachedTransactionMetadata(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) *TransactionMetadata) (cachedTransactionMetadata *objectstorage.CachedObject[*TransactionMetadata]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.transactionMetadataStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) *TransactionMetadata {
			return computeIfAbsentCallback[0](transactionID)
		})
	}

	return s.transactionMetadataStorage.Load(transactionID.Bytes())
}

// CachedOutput retrieves the Output with the given OutputID from the object storage.
func (s *storage) CachedOutput(outputID utxo.OutputID) (cachedOutput *objectstorage.CachedObject[*Output]) {
	return s.outputStorage.Load(outputID.Bytes())
}

func (s *storage) CachedOutputs(outputIDs utxo.OutputIDs) (cachedOutputs objectstorage.CachedObjects[*Output]) {
	cachedOutputs = make(objectstorage.CachedObjects[*Output], 0)
	_ = outputIDs.ForEach(func(outputID utxo.OutputID) error {
		cachedOutputs = append(cachedOutputs, s.CachedOutput(outputID))
		return nil
	})

	return cachedOutputs
}

// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
func (s *storage) CachedOutputMetadata(outputID utxo.OutputID) (cachedOutputMetadata *objectstorage.CachedObject[*OutputMetadata]) {
	return s.outputMetadataStorage.Load(outputID.Bytes())
}

func (s *storage) CachedOutputsMetadata(outputIDs utxo.OutputIDs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], 0)
	_ = outputIDs.ForEach(func(outputID utxo.OutputID) error {
		cachedOutputsMetadata = append(cachedOutputsMetadata, s.CachedOutputMetadata(outputID))
		return nil
	})

	return cachedOutputsMetadata
}

// CachedConsumer retrieves the OutputMetadata with the given OutputID from the object storage.
func (s *storage) CachedConsumer(outputID utxo.OutputID, txID utxo.TransactionID, computeIfAbsentCallback ...func(outputID utxo.OutputID, txID utxo.TransactionID) *Consumer) (cachedOutput *objectstorage.CachedObject[*Consumer]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.consumerStorage.ComputeIfAbsent(byteutils.ConcatBytes(outputID.Bytes(), txID.Bytes()), func(key []byte) *Consumer {
			return computeIfAbsentCallback[0](outputID, txID)
		})
	}

	return s.consumerStorage.Load(byteutils.ConcatBytes(outputID.Bytes(), txID.Bytes()))
}

// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
func (s *storage) CachedConsumers(outputID utxo.OutputID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	s.consumerStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Consumer]) bool {
		cachedConsumers = append(cachedConsumers, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(outputID.Bytes()))

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

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
