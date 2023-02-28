package ledger

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/cerrors"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
	"github.com/iotaledger/hive.go/core/generics/dataflow"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/objectstorage/generic"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage is a Ledger component that bundles the storage related API.
type Storage struct {
	// transactionStorage is an object storage used to persist Transactions objects.
	transactionStorage *generic.ObjectStorage[utxo.Transaction]

	// transactionMetadataStorage is an object storage used to persist TransactionMetadata objects.
	transactionMetadataStorage *generic.ObjectStorage[*TransactionMetadata]

	// OutputStorage is an object storage used to persist Output objects.
	OutputStorage *generic.ObjectStorage[utxo.Output]

	// OutputMetadataStorage is an object storage used to persist OutputMetadata objects.
	OutputMetadataStorage *generic.ObjectStorage[*OutputMetadata]

	// consumerStorage is an object storage used to persist Consumer objects.
	consumerStorage *generic.ObjectStorage[*Consumer]

	// ledger contains a reference to the Ledger that created the storage.
	ledger *Ledger

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

// newStorage returns a new storage instance for the given Ledger.
func newStorage(ledger *Ledger, baseStore kvstore.KVStore) (storage *Storage) {
	storage = &Storage{
		transactionStorage: generic.NewInterfaceStorage[utxo.Transaction](
			lo.PanicOnErr(baseStore.WithExtendedRealm([]byte{database.PrefixLedger, PrefixTransactionStorage})),
			transactionFactory(ledger.optsVM),
			ledger.optsCacheTimeProvider.CacheTime(ledger.optsTransactionCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		transactionMetadataStorage: generic.NewStructStorage[TransactionMetadata](
			lo.PanicOnErr(baseStore.WithExtendedRealm([]byte{database.PrefixLedger, PrefixTransactionMetadataStorage})),
			ledger.optsCacheTimeProvider.CacheTime(ledger.optTransactionMetadataCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		OutputStorage: generic.NewInterfaceStorage[utxo.Output](
			lo.PanicOnErr(baseStore.WithExtendedRealm([]byte{database.PrefixLedger, PrefixOutputStorage})),
			outputFactory(ledger.optsVM),
			ledger.optsCacheTimeProvider.CacheTime(ledger.optsOutputCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		OutputMetadataStorage: generic.NewStructStorage[OutputMetadata](
			lo.PanicOnErr(baseStore.WithExtendedRealm([]byte{database.PrefixLedger, PrefixOutputMetadataStorage})),
			ledger.optsCacheTimeProvider.CacheTime(ledger.optsOutputMetadataCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		consumerStorage: generic.NewStructStorage[Consumer](
			lo.PanicOnErr(baseStore.WithExtendedRealm([]byte{database.PrefixLedger, PrefixConsumerStorage})),
			ledger.optsCacheTimeProvider.CacheTime(ledger.optsConsumerCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.PartitionKey(new(Consumer).KeyPartitions()...),
		),
		ledger: ledger,
	}
	return storage
}

// CachedTransaction retrieves the CachedObject representing the named Transaction. The optional computeIfAbsentCallback
// can be used to dynamically initialize a non-existing Transaction.
func (s *Storage) CachedTransaction(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) utxo.Transaction) (cachedTransaction *generic.CachedObject[utxo.Transaction]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.transactionStorage.ComputeIfAbsent(lo.PanicOnErr(transactionID.Bytes()), func(key []byte) utxo.Transaction {
			return computeIfAbsentCallback[0](transactionID)
		})
	}

	return s.transactionStorage.Load(lo.PanicOnErr(transactionID.Bytes()))
}

// CachedTransactionMetadata retrieves the CachedObject representing the named TransactionMetadata. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing TransactionMetadata.
func (s *Storage) CachedTransactionMetadata(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) *TransactionMetadata) (cachedTransactionMetadata *generic.CachedObject[*TransactionMetadata]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.transactionMetadataStorage.ComputeIfAbsent(lo.PanicOnErr(transactionID.Bytes()), func(key []byte) *TransactionMetadata {
			return computeIfAbsentCallback[0](transactionID)
		})
	}

	return s.transactionMetadataStorage.Load(lo.PanicOnErr(transactionID.Bytes()))
}

// CachedOutput retrieves the CachedObject representing the named Output. The optional computeIfAbsentCallback can be
// used to dynamically initialize a non-existing Output.
func (s *Storage) CachedOutput(outputID utxo.OutputID, computeIfAbsentCallback ...func(outputID utxo.OutputID) utxo.Output) (cachedOutput *generic.CachedObject[utxo.Output]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.OutputStorage.ComputeIfAbsent(lo.PanicOnErr(outputID.Bytes()), func(key []byte) utxo.Output {
			return computeIfAbsentCallback[0](outputID)
		})
	}

	return s.OutputStorage.Load(lo.PanicOnErr(outputID.Bytes()))
}

// CachedOutputs retrieves the CachedObjects containing the named Outputs.
func (s *Storage) CachedOutputs(outputIDs utxo.OutputIDs) (cachedOutputs generic.CachedObjects[utxo.Output]) {
	cachedOutputs = make(generic.CachedObjects[utxo.Output], 0)
	for it := outputIDs.Iterator(); it.HasNext(); {
		cachedOutputs = append(cachedOutputs, s.CachedOutput(it.Next()))
	}

	return cachedOutputs
}

// CachedOutputMetadata retrieves the CachedObject representing the named OutputMetadata. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing OutputMetadata.
func (s *Storage) CachedOutputMetadata(outputID utxo.OutputID, computeIfAbsentCallback ...func(outputID utxo.OutputID) *OutputMetadata) (cachedOutputMetadata *generic.CachedObject[*OutputMetadata]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.OutputMetadataStorage.ComputeIfAbsent(lo.PanicOnErr(outputID.Bytes()), func(key []byte) *OutputMetadata {
			return computeIfAbsentCallback[0](outputID)
		})
	}

	return s.OutputMetadataStorage.Load(lo.PanicOnErr(outputID.Bytes()))
}

// CachedOutputsMetadata retrieves the CachedObjects containing the named OutputMetadata.
func (s *Storage) CachedOutputsMetadata(outputIDs utxo.OutputIDs) (cachedOutputsMetadata generic.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(generic.CachedObjects[*OutputMetadata], 0)
	for it := outputIDs.Iterator(); it.HasNext(); {
		cachedOutputsMetadata = append(cachedOutputsMetadata, s.CachedOutputMetadata(it.Next()))
	}

	return cachedOutputsMetadata
}

// CachedConsumer retrieves the CachedObject representing the named Consumer. The optional computeIfAbsentCallback can
// be used to dynamically initialize a non-existing Consumer.
func (s *Storage) CachedConsumer(outputID utxo.OutputID, txID utxo.TransactionID, computeIfAbsentCallback ...func(outputID utxo.OutputID, txID utxo.TransactionID) *Consumer) (cachedConsumer *generic.CachedObject[*Consumer]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.consumerStorage.ComputeIfAbsent(byteutils.ConcatBytes(lo.PanicOnErr(outputID.Bytes()), lo.PanicOnErr(txID.Bytes())), func(key []byte) *Consumer {
			return computeIfAbsentCallback[0](outputID, txID)
		})
	}

	return s.consumerStorage.Load(byteutils.ConcatBytes(lo.PanicOnErr(outputID.Bytes()), lo.PanicOnErr(txID.Bytes())))
}

// CachedConsumers retrieves the CachedObjects containing the named Consumers.
func (s *Storage) CachedConsumers(outputID utxo.OutputID) (cachedConsumers generic.CachedObjects[*Consumer]) {
	cachedConsumers = make(generic.CachedObjects[*Consumer], 0)
	s.consumerStorage.ForEach(func(key []byte, cachedObject *generic.CachedObject[*Consumer]) bool {
		cachedConsumers = append(cachedConsumers, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(lo.PanicOnErr(outputID.Bytes())))

	return
}

func (s *Storage) ForEachOutputID(callback func(utxo.OutputID) bool) {
	s.OutputStorage.ForEach(func(key []byte, _ *generic.CachedObject[utxo.Output]) bool {
		outputID := new(utxo.OutputID)
		_ = lo.PanicOnErr(outputID.FromBytes(key))
		return callback(*outputID)
	})
}

// Prune resets the database and deletes all entities.
func (s *Storage) Prune() (err error) {
	for _, storagePrune := range []func() error{
		s.transactionStorage.Prune,
		s.transactionMetadataStorage.Prune,
		s.OutputStorage.Prune,
		s.OutputMetadataStorage.Prune,
		s.consumerStorage.Prune,
	} {
		if err = storagePrune(); err != nil {
			err = errors.WithMessagef(cerrors.ErrFatal, "failed to prune the object storage (%v)", err)
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
		s.OutputStorage.Shutdown()
		s.OutputMetadataStorage.Shutdown()
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

		return errors.WithMessagef(ErrTransactionUnsolid, "%s is an unsolid reattachment", params.Transaction.ID())
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
func (s *Storage) initConsumers(outputIDs utxo.OutputIDs, txID utxo.TransactionID) (cachedConsumers generic.CachedObjects[*Consumer]) {
	cachedConsumers = make(generic.CachedObjects[*Consumer], 0)
	for it := outputIDs.Iterator(); it.HasNext(); {
		cachedConsumers = append(cachedConsumers, s.CachedConsumer(it.Next(), txID, NewConsumer))
	}

	return cachedConsumers
}

// pruneTransaction removes a Transaction (and all of its dependencies) from the database.
func (s *Storage) pruneTransaction(txID utxo.TransactionID, pruneFutureCone bool) {
	for futureConeWalker := walker.New[utxo.TransactionID]().Push(txID); futureConeWalker.HasNext(); {
		spentOutputsWithMetadata := make([]*OutputWithMetadata, 0)

		currentTxID := futureConeWalker.Next()

		s.CachedTransactionMetadata(currentTxID).Consume(func(txMetadata *TransactionMetadata) {
			s.CachedTransaction(currentTxID).Consume(func(tx utxo.Transaction) {
				for it := s.ledger.Utils.ResolveInputs(tx.Inputs()).Iterator(); it.HasNext(); {
					inputID := it.Next()

					s.CachedOutput(inputID).Consume(func(output utxo.Output) {
						s.CachedOutputMetadata(inputID).Consume(func(outputMetadata *OutputMetadata) {
							outputWithMetadata := NewOutputWithMetadata(outputMetadata.InclusionSlot(), outputMetadata.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())
							// TODO: we need to set the spent slot here
							spentOutputsWithMetadata = append(spentOutputsWithMetadata, outputWithMetadata)
						})
					})

					s.consumerStorage.Delete(byteutils.ConcatBytes(lo.PanicOnErr(inputID.Bytes()), lo.PanicOnErr(currentTxID.Bytes())))
				}
				tx.Delete()
			})

			createdOutputIDs := txMetadata.OutputIDs()
			createdOutputsWithMetadata := make([]*OutputWithMetadata, 0, createdOutputIDs.Size())
			for it := createdOutputIDs.Iterator(); it.HasNext(); {
				outputID := it.Next()
				outputIDBytes := lo.PanicOnErr(outputID.Bytes())

				s.CachedOutput(outputID).Consume(func(output utxo.Output) {
					s.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
						// TODO: inclusion slot is not set here
						createdOutputsWithMetadata = append(createdOutputsWithMetadata, NewOutputWithMetadata(outputMetadata.InclusionSlot(), outputMetadata.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID()))
					})
				})

				s.OutputStorage.Delete(outputIDBytes)
				s.OutputMetadataStorage.Delete(outputIDBytes)
			}

			txMetadata.Delete()

			s.ledger.Events.TransactionOrphaned.Trigger(&TransactionEvent{
				Metadata:       txMetadata,
				CreatedOutputs: createdOutputsWithMetadata,
				SpentOutputs:   spentOutputsWithMetadata,
			})

			if pruneFutureCone {
				s.ledger.Utils.WalkConsumingTransactionID(createdOutputIDs, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
					futureConeWalker.Push(consumingTxID)
				})
			}
		})
	}
}

// transactionFactory represents the object factory for the Transaction type.
func transactionFactory(vm vm.VM) func(key []byte, data []byte) (output generic.StorableObject, err error) {
	return func(key []byte, data []byte) (output generic.StorableObject, err error) {
		var txID utxo.TransactionID
		if _, err = txID.Decode(key); err != nil {
			panic(err)
		}

		parsedTransaction, err := vm.ParseTransaction(data)
		if err != nil {
			panic(err)
		}

		parsedTransaction.SetID(txID)

		return parsedTransaction, nil
	}
}

// outputFactory represents the object factory for the Output type.
func outputFactory(vm vm.VM) func(key []byte, data []byte) (output generic.StorableObject, err error) {
	return func(key []byte, data []byte) (output generic.StorableObject, err error) {
		var outputID utxo.OutputID
		if _, err = serix.DefaultAPI.Decode(context.Background(), key, &outputID, serix.WithValidation()); err != nil {
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
