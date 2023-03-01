package ledgerstate

import (
	"io"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/slot"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
)

const (
	spentOutputsPrefix byte = iota
	createdOutputsPrefix
)

// region StateDiffs ///////////////////////////////////////////////////////////////////////////////////////////////////

type StateDiffs struct {
	storage *storage.Storage
	ledger  *ledger.Ledger
}

func NewStateDiffs(storageInstance *storage.Storage, ledgerInstance *ledger.Ledger) (newLedgerStateDiffs *StateDiffs) {
	return &StateDiffs{
		storage: storageInstance,
		ledger:  ledgerInstance,
	}
}

func (s *StateDiffs) StoreSpentOutput(outputWithMetadata *ledger.OutputWithMetadata) (err error) {
	if spentStorage, spentStorageErr := s.storage.LedgerStateDiffs(outputWithMetadata.SpentInSlot()).WithExtendedRealm([]byte{spentOutputsPrefix}); spentStorageErr != nil {
		return errors.Wrapf(spentStorageErr, "failed to retrieve spent storage")
	} else {
		return spentStorage.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), lo.PanicOnErr(outputWithMetadata.Bytes()))
	}
}

func (s *StateDiffs) StoreCreatedOutput(outputWithMetadata *ledger.OutputWithMetadata) (err error) {
	if createdStorage, createdStorageErr := s.storage.LedgerStateDiffs(outputWithMetadata.Index()).WithExtendedRealm([]byte{createdOutputsPrefix}); createdStorageErr != nil {
		return errors.Wrapf(createdStorageErr, "failed to retrieve created storage")
	} else {
		return createdStorage.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), lo.PanicOnErr(outputWithMetadata.Bytes()))
	}
}

func (s *StateDiffs) LoadSpentOutput(index slot.Index, outputID utxo.OutputID) (outputWithMetadata *ledger.OutputWithMetadata, err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return nil, errors.Wrap(err, "failed to extend realm for storage")
	}
	return s.get(store, outputID)
}

func (s *StateDiffs) LoadCreatedOutput(index slot.Index, outputID utxo.OutputID) (outputWithMetadata *ledger.OutputWithMetadata, err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return nil, errors.Wrap(err, "failed to extend realm for storage")
	}

	return s.get(store, outputID)
}

func (s *StateDiffs) DeleteSpentOutput(index slot.Index, outputID utxo.OutputID) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return errors.Wrap(err, "failed to extend realm for storage")
	}

	return s.delete(store, outputID)
}

func (s *StateDiffs) DeleteCreatedOutput(index slot.Index, outputID utxo.OutputID) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return errors.Wrap(err, "failed to extend realm for storage")
	}

	return s.delete(store, outputID)
}

func (s *StateDiffs) DeleteSpentOutputs(index slot.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = s.DeleteSpentOutput(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (s *StateDiffs) DeleteCreatedOutputs(index slot.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = s.DeleteCreatedOutput(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (s *StateDiffs) StreamSpentOutputs(index slot.Index, callback func(*ledger.OutputWithMetadata) error) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return errors.Wrap(err, "failed to extend realm for storage")
	}

	return s.stream(store, callback)
}

func (s *StateDiffs) StreamCreatedOutputs(index slot.Index, callback func(*ledger.OutputWithMetadata) error) (err error) {
	store, err := s.storage.LedgerStateDiffs(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return errors.Wrap(err, "failed to extend realm for storage")
	}

	return s.stream(store, callback)
}

func (s *StateDiffs) Export(writer io.WriteSeeker, targetSlot slot.Index) (err error) {
	if targetSlot > s.storage.Settings.LatestCommitment().Index() {
		return errors.Errorf("target slot %d is not yet committed", targetSlot)
	}

	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		for currentSlot := s.storage.Settings.LatestCommitment().Index(); currentSlot > targetSlot; currentSlot-- {
			if err = stream.Write(writer, uint64(currentSlot)); err != nil {
				return 0, errors.Wrapf(err, "failed to write slot %d", currentSlot)
			} else if err = s.exportOutputs(writer, currentSlot, s.StreamCreatedOutputs); err != nil {
				return 0, errors.Wrapf(err, "failed to export created outputs for slot %d", currentSlot)
			} else if err = s.exportOutputs(writer, currentSlot, s.StreamSpentOutputs); err != nil {
				return 0, errors.Wrapf(err, "failed to export spent outputs for slot %d", currentSlot)
			}

			elementsCount++
		}

		return
	})
}

func (s *StateDiffs) Import(reader io.ReadSeeker) (importedSlots []slot.Index, err error) {
	if err = stream.ReadCollection(reader, func(i int) (err error) {
		slotIndex, err := stream.Read[uint64](reader)
		if err != nil {
			return errors.Wrap(err, "failed to read slot index")
		}
		importedSlots = append(importedSlots, slot.Index(slotIndex))

		if err = s.importOutputs(reader, s.StoreCreatedOutput); err != nil {
			return errors.Wrapf(err, "failed to import created outputs for slot %d", slotIndex)
		} else if err = s.importOutputs(reader, s.StoreSpentOutput); err != nil {
			return errors.Wrapf(err, "failed to import spent outputs for slot %d", slotIndex)
		}

		return
	}); err != nil {
		return nil, errors.Wrap(err, "failed to import state diffs")
	}

	return
}

func (s *StateDiffs) Delete(index slot.Index) (err error) {
	return s.storage.LedgerStateDiffs(index).Clear()
}

func (s *StateDiffs) importOutputs(reader io.ReadSeeker, store func(*ledger.OutputWithMetadata) error) (err error) {
	output := new(ledger.OutputWithMetadata)
	return stream.ReadCollection(reader, func(i int) (err error) {
		if err = stream.ReadSerializable(reader, output); err != nil {
			return errors.Wrapf(err, "failed to read output %d", i)
		} else if err = store(output); err != nil {
			return errors.Wrapf(err, "failed to store output %d", i)
		}

		return
	})
}

func (s *StateDiffs) exportOutputs(writer io.WriteSeeker, slot slot.Index, streamFunc func(index slot.Index, callback func(*ledger.OutputWithMetadata) error) (err error)) (err error) {
	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		if err = streamFunc(slot, func(outputWithMetadata *ledger.OutputWithMetadata) (err error) {
			if err = stream.WriteSerializable(writer, outputWithMetadata); err != nil {
				return errors.Wrap(err, "failed to write output")
			}

			elementsCount++

			return
		}); err != nil {
			return 0, errors.Wrap(err, "failed to stream outputs")
		}

		return
	})
}

func (s *StateDiffs) get(store kvstore.KVStore, outputID utxo.OutputID) (outputWithMetadata *ledger.OutputWithMetadata, err error) {
	outputWithMetadataBytes, err := store.Get(lo.PanicOnErr(outputID.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil
		}

		return nil, errors.Wrapf(err, "failed to get block %s", outputID)
	}

	outputWithMetadata = new(ledger.OutputWithMetadata)
	if _, err = outputWithMetadata.FromBytes(outputWithMetadataBytes); err != nil {
		return nil, errors.Wrapf(err, "failed to parse output with metadata %s", outputID)
	}
	outputWithMetadata.SetID(outputID)

	return
}

func (s *StateDiffs) stream(store kvstore.KVStore, callback func(*ledger.OutputWithMetadata) error) (err error) {
	if iterationErr := store.Iterate([]byte{}, func(idBytes kvstore.Key, outputWithMetadataBytes kvstore.Value) bool {
		outputID := new(utxo.OutputID)
		outputWithMetadata := new(ledger.OutputWithMetadata)

		if _, err = outputID.FromBytes(idBytes); err != nil {
			err = errors.Wrapf(err, "failed to parse output ID %s", idBytes)
		} else if _, err = outputWithMetadata.FromBytes(outputWithMetadataBytes); err != nil {
			err = errors.Wrapf(err, "failed to parse output with metadata %s", outputID)
		} else {
			outputWithMetadata.SetID(*outputID)
			err = callback(outputWithMetadata)
		}

		return err == nil
	}); iterationErr != nil {
		err = errors.Wrapf(iterationErr, "failed to iterate over outputs")
	}

	return
}

func (s *StateDiffs) delete(store kvstore.KVStore, outputID utxo.OutputID) (err error) {
	if err := store.Delete(lo.PanicOnErr(outputID.Bytes())); err != nil {
		return errors.Wrapf(err, "failed to delete output %s", outputID)
	}
	return
}

func (s *StateDiffs) addAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	if !s.ledger.Storage.CachedTransaction(metadata.ID()).Consume(func(transaction utxo.Transaction) {
		err = s.storeTransaction(transaction, metadata)
	}) {
		err = errors.Errorf("failed to get transaction %s from cache", metadata.ID())
	}

	return
}

func (s *StateDiffs) storeTransaction(transaction utxo.Transaction, metadata *ledger.TransactionMetadata) (err error) {
	for it := s.ledger.Utils.ResolveInputs(transaction.Inputs()).Iterator(); it.HasNext(); {
		inputWithMetadata := s.outputWithMetadata(it.Next())
		inputWithMetadata.SetSpentInSlot(metadata.InclusionSlot())
		if err = s.StoreSpentOutput(inputWithMetadata); err != nil {
			return errors.Wrap(err, "failed to storeOutput spent output")
		}
	}

	for it := metadata.OutputIDs().Iterator(); it.HasNext(); {
		if err = s.StoreCreatedOutput(s.outputWithMetadata(it.Next())); err != nil {
			return errors.Wrap(err, "failed to storeOutput created output")
		}
	}

	if metadata.InclusionSlot() > s.storage.Settings.LatestStateMutationSlot() {
		if err = s.storage.Settings.SetLatestStateMutationSlot(metadata.InclusionSlot()); err != nil {
			return errors.Wrap(err, "failed to update latest state mutation slot")
		}
	}

	return nil
}

func (s *StateDiffs) outputWithMetadata(outputID utxo.OutputID) (outputWithMetadata *ledger.OutputWithMetadata) {
	s.ledger.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
		s.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
			outputWithMetadata = ledger.NewOutputWithMetadata(outputMetadata.InclusionSlot(), outputID, output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())
		})
	})

	return
}

func (s *StateDiffs) moveTransactionToOtherSlot(txMeta *ledger.TransactionMetadata, oldSlot, newSlot slot.Index) {
	if oldSlot <= s.storage.Settings.LatestCommitment().Index() || newSlot <= s.storage.Settings.LatestCommitment().Index() {
		s.ledger.Events.Error.Trigger(errors.Errorf("inclusion time of transaction changed for already committed slot: previous Index %d, new Index %d", oldSlot, newSlot))
		return
	}

	s.ledger.Storage.CachedTransaction(txMeta.ID()).Consume(func(tx utxo.Transaction) {
		if err := s.DeleteSpentOutputs(oldSlot, s.ledger.Utils.ResolveInputs(tx.Inputs())); err != nil {
			s.ledger.Events.Error.Trigger(errors.Wrapf(err, "failed to delete spent outputs of transaction %s", txMeta.ID()))
		}

		if err := s.DeleteCreatedOutputs(oldSlot, txMeta.OutputIDs()); err != nil {
			s.ledger.Events.Error.Trigger(errors.Wrapf(err, "failed to delete created outputs of transaction %s", txMeta.ID()))
		}

		if err := s.storeTransaction(tx, txMeta); err != nil {
			s.ledger.Events.Error.Trigger(errors.Wrapf(err, "failed to store transaction %s when moving from slot %d to %d", txMeta.ID(), oldSlot, newSlot))
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
