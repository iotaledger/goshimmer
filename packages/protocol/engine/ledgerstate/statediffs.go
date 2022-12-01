package ledgerstate

import (
	"io"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/initializable"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage"
)

const (
	spentOutputsPrefix byte = iota
	createdOutputsPrefix
)

// region StateDiffs ///////////////////////////////////////////////////////////////////////////////////////////////////

type StateDiffs struct {
	storage *storage.Storage
	ledger  *ledger.Ledger

	*initializable.Initializable
}

func NewStateDiffs(storageInstance *storage.Storage) (newLedgerStateDiffs *StateDiffs) {
	return &StateDiffs{
		Initializable: initializable.New(),
		storage:       storageInstance,
	}
}

func (s *StateDiffs) StoreSpentOutput(outputWithMetadata *OutputWithMetadata) (err error) {
	if spentStorage, spentStorageErr := s.storage.LedgerStateDiffs(outputWithMetadata.Index()).WithExtendedRealm([]byte{spentOutputsPrefix}); spentStorageErr != nil {
		return errors.Errorf("failed to retrieve spent storage: %w", spentStorageErr)
	} else {
		return spentStorage.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), lo.PanicOnErr(outputWithMetadata.Bytes()))
	}
}

func (s *StateDiffs) StoreCreatedOutput(outputWithMetadata *OutputWithMetadata) (err error) {
	if createdStorage, createdStorageErr := s.storage.LedgerStateDiffs(outputWithMetadata.Index()).WithExtendedRealm([]byte{createdOutputsPrefix}); createdStorageErr != nil {
		return errors.Errorf("failed to retrieve created storage: %w", createdStorageErr)
	} else {
		return createdStorage.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), lo.PanicOnErr(outputWithMetadata.Bytes()))
	}
}

func (s *StateDiffs) LoadSpentOutput(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return nil, errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return s.get(store, outputID)
}

func (s *StateDiffs) LoadCreatedOutput(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return nil, errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.get(store, outputID)
}

func (s *StateDiffs) DeleteSpentOutput(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.delete(store, outputID)
}

func (s *StateDiffs) DeleteCreatedOutput(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.delete(store, outputID)
}

func (s *StateDiffs) DeleteSpentOutputs(index epoch.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = s.DeleteSpentOutput(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (s *StateDiffs) DeleteCreatedOutputs(index epoch.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = s.DeleteCreatedOutput(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (s *StateDiffs) StreamSpentOutputs(index epoch.Index, callback func(*OutputWithMetadata) error) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.stream(store, callback)
}

func (s *StateDiffs) StreamCreatedOutputs(index epoch.Index, callback func(*OutputWithMetadata) error) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	s.stream(store, callback)

	return
}

func (s *StateDiffs) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if targetEpoch > s.storage.Settings.LatestCommitment().Index() {
		return errors.Errorf("target epoch %d is not yet committed", targetEpoch)
	}

	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		for currentEpoch := s.storage.Settings.LatestCommitment().Index(); currentEpoch > targetEpoch; currentEpoch-- {
			if err = stream.Write(writer, uint64(currentEpoch)); err != nil {
				return 0, errors.Errorf("failed to write epoch %d: %w", currentEpoch, err)
			} else if err = s.exportOutputs(writer, currentEpoch, s.StreamCreatedOutputs); err != nil {
				return 0, errors.Errorf("failed to export created outputs for epoch %d: %w", currentEpoch, err)
			} else if err = s.exportOutputs(writer, currentEpoch, s.StreamSpentOutputs); err != nil {
				return 0, errors.Errorf("failed to export spent outputs for epoch %d: %w", currentEpoch, err)
			}

			elementsCount++
		}

		return
	})
}

func (s *StateDiffs) Import(reader io.ReadSeeker) (importedEpochs []epoch.Index, err error) {
	if err = stream.ReadCollection(reader, func(i int) (err error) {
		epochIndex, err := stream.Read[uint64](reader)
		if err != nil {
			return errors.Errorf("failed to read epoch index: %w", err)
		}
		importedEpochs = append(importedEpochs, epoch.Index(epochIndex))

		if err = s.importOutputs(reader, s.StoreCreatedOutput); err != nil {
			return errors.Errorf("failed to import created outputs for epoch %d: %w", epochIndex, err)
		} else if err = s.importOutputs(reader, s.StoreSpentOutput); err != nil {
			return errors.Errorf("failed to import spent outputs for epoch %d: %w", epochIndex, err)
		}

		return
	}); err != nil {
		return nil, errors.Errorf("failed to import state diffs: %w", err)
	}

	s.TriggerInitialized()

	return
}

func (s *StateDiffs) importOutputs(reader io.ReadSeeker, store func(*OutputWithMetadata) error) (err error) {
	output := new(OutputWithMetadata)
	return stream.ReadCollection(reader, func(i int) (err error) {
		if err = stream.ReadSerializable(reader, output); err != nil {
			return errors.Errorf("failed to read output %d: %w", i, err)
		} else if err = store(output); err != nil {
			return errors.Errorf("failed to store output %d: %w", i, err)
		}

		return
	})
}

func (s *StateDiffs) exportOutputs(writer io.WriteSeeker, epoch epoch.Index, streamFunc func(index epoch.Index, callback func(*OutputWithMetadata) error) (err error)) (err error) {
	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		if err = streamFunc(epoch, func(output *OutputWithMetadata) (err error) {
			if err = stream.WriteSerializable(writer, output); err != nil {
				return errors.Errorf("failed to write output: %w", err)
			}

			elementsCount++

			return
		}); err != nil {
			return 0, errors.Errorf("failed to stream outputs: %w", err)
		}

		return
	})
}

func (s *StateDiffs) get(store kvstore.KVStore, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	outputWithMetadataBytes, err := store.Get(lo.PanicOnErr(outputID.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil
		}

		return nil, errors.Errorf("failed to get block %s: %w", outputID, err)
	}

	outputWithMetadata = new(OutputWithMetadata)
	if _, err = outputWithMetadata.FromBytes(outputWithMetadataBytes); err != nil {
		return nil, errors.Errorf("failed to parse output with metadata %s: %w", outputID, err)
	}
	outputWithMetadata.SetID(outputID)

	return
}

func (s *StateDiffs) stream(store kvstore.KVStore, callback func(*OutputWithMetadata) error) (err error) {
	if iterationErr := store.Iterate([]byte{}, func(idBytes kvstore.Key, outputWithMetadataBytes kvstore.Value) bool {
		outputID := new(utxo.OutputID)
		outputWithMetadata := new(OutputWithMetadata)

		if _, err = outputID.FromBytes(idBytes); err != nil {
			err = errors.Errorf("failed to parse output ID %s: %w", idBytes, err)
		} else if _, err = outputWithMetadata.FromBytes(outputWithMetadataBytes); err != nil {
			err = errors.Errorf("failed to parse output with metadata %s: %w", outputID, err)
		} else {
			outputWithMetadata.SetID(*outputID)
			err = callback(outputWithMetadata)
		}

		return err == nil
	}); iterationErr != nil {
		err = errors.Errorf("failed to iterate over outputs: %w", iterationErr)
	}

	return
}

func (s *StateDiffs) delete(store kvstore.KVStore, outputID utxo.OutputID) (err error) {
	if err := store.Delete(lo.PanicOnErr(outputID.Bytes())); err != nil {
		return errors.Errorf("failed to delete output %s: %w", outputID, err)
	}
	return
}

func (s *StateDiffs) addAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	if !s.ledger.Storage.CachedTransaction(metadata.ID()).Consume(func(transaction utxo.Transaction) {
		err = s.storeTransaction(epoch.IndexFromTime(metadata.InclusionTime()), transaction, metadata)
	}) {
		err = errors.Errorf("failed to get transaction %s from cache", metadata.ID())
	}

	return
}

func (s *StateDiffs) storeTransaction(index epoch.Index, transaction utxo.Transaction, metadata *ledger.TransactionMetadata) (err error) {
	for it := s.ledger.Utils.ResolveInputs(transaction.Inputs()).Iterator(); it.HasNext(); {
		if err = s.StoreSpentOutput(s.outputWithMetadata(index, it.Next())); err != nil {
			return errors.Errorf("failed to storeOutput spent output: %w", err)
		}
	}

	for it := metadata.OutputIDs().Iterator(); it.HasNext(); {
		if err = s.StoreCreatedOutput(s.outputWithMetadata(index, it.Next())); err != nil {
			return errors.Errorf("failed to storeOutput created output: %w", err)
		}
	}

	if index > s.ledger.ChainStorage.Settings.LatestStateMutationEpoch() {
		if err = s.ledger.ChainStorage.Settings.SetLatestStateMutationEpoch(index); err != nil {
			return errors.Errorf("failed to update latest state mutation epoch: %w", err)
		}
	}

	return nil
}

func (s *StateDiffs) outputWithMetadata(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata) {
	s.ledger.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
		s.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
			outputWithMetadata = NewOutputWithMetadata(index, outputID, output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())
		})
	})

	return
}

func (s *StateDiffs) moveTransactionToOtherEpoch(txMeta *ledger.TransactionMetadata, previousInclusionTime, inclusionTime time.Time) {
	oldEpoch := epoch.IndexFromTime(previousInclusionTime)
	newEpoch := epoch.IndexFromTime(inclusionTime)

	if oldEpoch == 0 || oldEpoch == newEpoch {
		return
	}

	if oldEpoch <= s.ledger.ChainStorage.Settings.LatestCommitment().Index() || newEpoch <= s.ledger.ChainStorage.Settings.LatestCommitment().Index() {
		s.ledger.Events.Error.Trigger(errors.Errorf("inclusion time of transaction changed for already committed epoch: previous Index %d, new Index %d", oldEpoch, newEpoch))
		return
	}

	s.ledger.Storage.CachedTransaction(txMeta.ID()).Consume(func(tx utxo.Transaction) {
		s.DeleteSpentOutputs(oldEpoch, s.ledger.Utils.ResolveInputs(tx.Inputs()))
	})

	s.DeleteCreatedOutputs(oldEpoch, txMeta.OutputIDs())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
