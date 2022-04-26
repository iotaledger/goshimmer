package ledger

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage is a Ledger component that bundles the storage related API.
type Storage struct {
	// transactionStorage is an object storage used to persist Transactions objects.
	transactionStorage *objectstorage.ObjectStorage[*Transaction]

	// transactionMetadataStorage is an object storage used to persist TransactionMetadata objects.
	transactionMetadataStorage *objectstorage.ObjectStorage[*TransactionMetadata]

	// outputStorage is an object storage used to persist Output objects.
	outputStorage *objectstorage.ObjectStorage[utxo.Output]

	// outputMetadataStorage is an object storage used to persist OutputMetadata objects.
	outputMetadataStorage *objectstorage.ObjectStorage[*OutputMetadata]

	// consumerStorage is an object storage used to persist Consumer objects.
	consumerStorage *objectstorage.ObjectStorage[*Consumer]

	// ledger contains a reference to the Ledger that created the storage.
	ledger *Ledger

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

// newStorage returns a new storage instance for the given Ledger.
func newStorage(ledger *Ledger) (new *Storage) {
	return &Storage{
		transactionStorage: objectstorage.New[*Transaction](
			ledger.options.store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionStorage}),
			ledger.options.cacheTimeProvider.CacheTime(ledger.options.transactionCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
			objectstorage.WithObjectFactory(transactionFactory(ledger.options.vm)),
		),
		transactionMetadataStorage: objectstorage.New[*TransactionMetadata](
			ledger.options.store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionMetadataStorage}),
			ledger.options.cacheTimeProvider.CacheTime(ledger.options.transactionMetadataCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		outputStorage: objectstorage.New[utxo.Output](
			ledger.options.store.WithRealm([]byte{database.PrefixLedger, PrefixOutputStorage}),
			ledger.options.cacheTimeProvider.CacheTime(ledger.options.outputCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
			objectstorage.WithObjectFactory(outputFactory(ledger.options.vm)),
		),
		outputMetadataStorage: objectstorage.New[*OutputMetadata](
			ledger.options.store.WithRealm([]byte{database.PrefixLedger, PrefixOutputMetadataStorage}),
			ledger.options.cacheTimeProvider.CacheTime(ledger.options.outputMetadataCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		consumerStorage: objectstorage.New[*Consumer](
			ledger.options.store.WithRealm([]byte{database.PrefixLedger, PrefixConsumerStorage}),
			ledger.options.cacheTimeProvider.CacheTime(ledger.options.consumerCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			consumerPartitionKeys,
		),
		ledger: ledger,
	}
}

// CachedTransaction retrieves the CachedObject representing the named Transaction. The optional computeIfAbsentCallback
// can be used to dynamically initialize a non-existing Transaction.
func (s *Storage) CachedTransaction(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) *Transaction) (cachedTransaction *objectstorage.CachedObject[*Transaction]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.transactionStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) *Transaction {
			return computeIfAbsentCallback[0](transactionID)
		})
	}

	return s.transactionStorage.Load(transactionID.Bytes())
}

// CachedTransactionMetadata retrieves the CachedObject representing the named TransactionMetadata. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing TransactionMetadata.
func (s *Storage) CachedTransactionMetadata(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) *TransactionMetadata) (cachedTransactionMetadata *objectstorage.CachedObject[*TransactionMetadata]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.transactionMetadataStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) *TransactionMetadata {
			return computeIfAbsentCallback[0](transactionID)
		})
	}

	return s.transactionMetadataStorage.Load(transactionID.Bytes())
}

// CachedOutput retrieves the CachedObject representing the named Output. The optional computeIfAbsentCallback can be
// used to dynamically initialize a non-existing Output.
func (s *Storage) CachedOutput(outputID utxo.OutputID, computeIfAbsentCallback ...func(outputID utxo.OutputID) utxo.Output) (cachedOutput *objectstorage.CachedObject[utxo.Output]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.outputStorage.ComputeIfAbsent(outputID.Bytes(), func(key []byte) utxo.Output {
			return computeIfAbsentCallback[0](outputID)
		})
	}

	return s.outputStorage.Load(outputID.Bytes())
}

// CachedOutputs retrieves the CachedObjects containing the named Outputs.
func (s *Storage) CachedOutputs(outputIDs utxo.OutputIDs) (cachedOutputs objectstorage.CachedObjects[utxo.Output]) {
	cachedOutputs = make(objectstorage.CachedObjects[utxo.Output], 0)
	for it := outputIDs.Iterator(); it.HasNext(); {
		cachedOutputs = append(cachedOutputs, s.CachedOutput(it.Next()))
	}

	return cachedOutputs
}

// CachedOutputMetadata retrieves the CachedObject representing the named OutputMetadata. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing OutputMetadata.
func (s *Storage) CachedOutputMetadata(outputID utxo.OutputID, computeIfAbsentCallback ...func(outputID utxo.OutputID) *OutputMetadata) (cachedOutputMetadata *objectstorage.CachedObject[*OutputMetadata]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.outputMetadataStorage.ComputeIfAbsent(outputID.Bytes(), func(key []byte) *OutputMetadata {
			return computeIfAbsentCallback[0](outputID)
		})
	}

	return s.outputMetadataStorage.Load(outputID.Bytes())
}

// CachedOutputsMetadata retrieves the CachedObjects containing the named OutputMetadata.
func (s *Storage) CachedOutputsMetadata(outputIDs utxo.OutputIDs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], 0)
	for it := outputIDs.Iterator(); it.HasNext(); {
		cachedOutputsMetadata = append(cachedOutputsMetadata, s.CachedOutputMetadata(it.Next()))
	}

	return cachedOutputsMetadata
}

// CachedConsumer retrieves the CachedObject representing the named Consumer. The optional computeIfAbsentCallback can
// be used to dynamically initialize a non-existing Consumer.
func (s *Storage) CachedConsumer(outputID utxo.OutputID, txID utxo.TransactionID, computeIfAbsentCallback ...func(outputID utxo.OutputID, txID utxo.TransactionID) *Consumer) (cachedConsumer *objectstorage.CachedObject[*Consumer]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.consumerStorage.ComputeIfAbsent(byteutils.ConcatBytes(outputID.Bytes(), txID.Bytes()), func(key []byte) *Consumer {
			return computeIfAbsentCallback[0](outputID, txID)
		})
	}

	return s.consumerStorage.Load(byteutils.ConcatBytes(outputID.Bytes(), txID.Bytes()))
}

// CachedConsumers retrieves the CachedObjects containing the named Consumers.
func (s *Storage) CachedConsumers(outputID utxo.OutputID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	s.consumerStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Consumer]) bool {
		cachedConsumers = append(cachedConsumers, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(outputID.Bytes()))

	return
}

// Prune resets the database and deletes all entities.
func (s *Storage) Prune() (err error) {
	for _, storagePrune := range []func() error{
		s.transactionStorage.Prune,
		s.transactionMetadataStorage.Prune,
		s.outputStorage.Prune,
		s.outputMetadataStorage.Prune,
		s.consumerStorage.Prune,
	} {
		if err = storagePrune(); err != nil {
			err = errors.Errorf("failed to prune the object storage (%v): %w", err, cerrors.ErrFatal)
			return
		}
	}

	return
}

// Shutdown shuts down the KVStores used to persist data.
func (s *Storage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.transactionStorage.Shutdown()
		s.transactionMetadataStorage.Shutdown()
		s.outputStorage.Shutdown()
		s.outputMetadataStorage.Shutdown()
		s.consumerStorage.Shutdown()
	})
}

// storeTransactionCommand is a ChainedCommand that stores a Transaction.
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
		if params.TransactionMetadata.IsBooked() {
			return nil
		}

		return errors.Errorf("%s is an unsolid reattachment: %w", params.Transaction.ID(), ErrTransactionUnsolid)
	}

	params.InputIDs = s.ledger.Utils.ResolveInputs(params.Transaction.Inputs())

	cachedConsumers := s.initConsumers(params.InputIDs, params.Transaction.ID())
	defer cachedConsumers.Release()
	params.Consumers = cachedConsumers.Unwrap(true)

	s.ledger.Events.TransactionStored.Trigger(&TransactionStoredEvent{
		TransactionID: params.Transaction.ID(),
	})

	return next(params)
}

// initConsumers creates the Consumers of a Transaction if they didn't exist before.
func (s *Storage) initConsumers(outputIDs utxo.OutputIDs, txID utxo.TransactionID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	for it := outputIDs.Iterator(); it.HasNext(); {
		cachedConsumers = append(cachedConsumers, s.CachedConsumer(it.Next(), txID, NewConsumer))
	}

	return cachedConsumers
}

// pruneTransaction removes a Transaction (and all of its dependencies) from the database.
func (s *Storage) pruneTransaction(txID utxo.TransactionID, pruneFutureCone bool) {
	for futureConeWalker := walker.New[utxo.TransactionID]().Push(txID); futureConeWalker.HasNext(); {
		currentTxID := futureConeWalker.Next()

		s.CachedTransactionMetadata(currentTxID).Consume(func(txMetadata *TransactionMetadata) {
			s.CachedTransaction(currentTxID).Consume(func(tx *Transaction) {
				for it := s.ledger.Utils.ResolveInputs(tx.Inputs()).Iterator(); it.HasNext(); {
					s.consumerStorage.Delete(byteutils.ConcatBytes(it.Next().Bytes(), currentTxID.Bytes()))
				}

				tx.Delete()
			})

			createdOutputIDs := txMetadata.OutputIDs()
			for it := createdOutputIDs.Iterator(); it.HasNext(); {
				outputIDBytes := it.Next().Bytes()

				s.outputStorage.Delete(outputIDBytes)
				s.outputMetadataStorage.Delete(outputIDBytes)

			}

			if pruneFutureCone {
				s.ledger.Utils.WalkConsumingTransactionID(createdOutputIDs, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
					futureConeWalker.Push(consumingTxID)
				})
			}

			txMetadata.Delete()
		})
	}
}

// transactionFactory represents the object factory for the Transaction type.
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

// outputFactory represents the object factory for the Output type.
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

		return parsedOutput, nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixTransactionStorage defines the storage prefix for the Transaction object storage.
	PrefixTransactionStorage byte = iota

	// PrefixTransactionMetadataStorage defines the storage prefix for the TransactionMetadata object storage.
	PrefixTransactionMetadataStorage

	// PrefixOutputStorage defines the storage prefix for the Output object storage.
	PrefixOutputStorage

	// PrefixOutputMetadataStorage defines the storage prefix for the OutputMetadata object storage.
	PrefixOutputMetadataStorage

	// PrefixConsumerStorage defines the storage prefix for the Consumer object storage.
	PrefixConsumerStorage
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
