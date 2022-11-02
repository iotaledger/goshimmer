package prunable

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

const (
	spentOutputsPrefix byte = iota
	createdOutputsPrefix
)

// region StateDiffs ///////////////////////////////////////////////////////////////////////////////////////////////////

type LedgerStateDiffs struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

func NewLedgerStateDiffs(database *database.Manager, storagePrefix byte) (newLedgerStateDiffs *LedgerStateDiffs) {
	return &LedgerStateDiffs{
		Storage: lo.Bind([]byte{storagePrefix}, database.Get),
	}
}

func (s *LedgerStateDiffs) StoreSpentOutput(outputWithMetadata *models.OutputWithMetadata) (err error) {
	store, err := s.SpentStorage(outputWithMetadata.Index())
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return s.store(store, outputWithMetadata)
}

func (s *LedgerStateDiffs) StoreCreatedOutput(outputWithMetadata *models.OutputWithMetadata) (err error) {
	store, err := s.CreatedStorage(outputWithMetadata.Index())
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return s.store(store, outputWithMetadata)
}

func (s *LedgerStateDiffs) LoadSpentOutput(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *models.OutputWithMetadata, err error) {
	store, err := s.SpentStorage(index)
	if err != nil {
		return nil, errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return s.get(store, outputID)
}

func (s *LedgerStateDiffs) LoadCreatedOutput(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *models.OutputWithMetadata, err error) {
	store, err := s.CreatedStorage(index)
	if err != nil {
		return nil, errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.get(store, outputID)
}

func (s *LedgerStateDiffs) DeleteSpentOutput(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := s.SpentStorage(index)
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.delete(store, outputID)
}

func (s *LedgerStateDiffs) DeleteCreatedOutput(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := s.CreatedStorage(index)
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.delete(store, outputID)
}

func (s *LedgerStateDiffs) DeleteSpentOutputs(index epoch.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = s.DeleteSpentOutput(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (s *LedgerStateDiffs) DeleteCreatedOutputs(index epoch.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = s.DeleteCreatedOutput(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (s *LedgerStateDiffs) StreamSpentOutputs(index epoch.Index, callback func(*models.OutputWithMetadata)) (err error) {
	store, err := s.SpentStorage(index)
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	s.stream(store, callback)

	return
}

func (s *LedgerStateDiffs) StreamCreatedOutputs(index epoch.Index, callback func(*models.OutputWithMetadata)) (err error) {
	store, err := s.CreatedStorage(index)
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	s.stream(store, callback)

	return
}

func (s *LedgerStateDiffs) StateDiff(index epoch.Index) (diff *models.StateDiff) {
	diff = models.NewMemoryStateDiff()
	s.StreamCreatedOutputs(index, diff.ApplyCreatedOutput)
	s.StreamSpentOutputs(index, diff.ApplyDeletedOutput)

	return diff
}

func (s *LedgerStateDiffs) SpentStorage(index epoch.Index) (storage kvstore.KVStore, err error) {
	return s.Storage(index).WithExtendedRealm([]byte{spentOutputsPrefix})
}

func (s *LedgerStateDiffs) CreatedStorage(index epoch.Index) (storage kvstore.KVStore, err error) {
	return s.Storage(index).WithExtendedRealm([]byte{createdOutputsPrefix})
}

func (s *LedgerStateDiffs) store(store kvstore.KVStore, outputWithMetadata *models.OutputWithMetadata) (err error) {
	outputWithMetadataBytes := lo.PanicOnErr(outputWithMetadata.Bytes())
	if err := store.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), outputWithMetadataBytes); err != nil {
		return errors.Errorf("failed to store output with metadata %s: %w", outputWithMetadata.ID(), err)
	}
	return
}

func (s *LedgerStateDiffs) get(store kvstore.KVStore, outputID utxo.OutputID) (outputWithMetadata *models.OutputWithMetadata, err error) {
	outputWithMetadataBytes, err := store.Get(lo.PanicOnErr(outputID.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil
		}

		return nil, errors.Errorf("failed to get block %s: %w", outputID, err)
	}

	outputWithMetadata = new(models.OutputWithMetadata)
	if _, err = outputWithMetadata.FromBytes(outputWithMetadataBytes); err != nil {
		return nil, errors.Errorf("failed to parse output with metadata %s: %w", outputID, err)
	}
	outputWithMetadata.SetID(outputID)

	return
}

func (s *LedgerStateDiffs) stream(store kvstore.KVStore, callback func(*models.OutputWithMetadata)) {
	store.Iterate([]byte{}, func(idBytes kvstore.Key, outputWithMetadataBytes kvstore.Value) bool {
		outputID := new(utxo.OutputID)
		outputID.FromBytes(idBytes)
		outputWithMetadata := new(models.OutputWithMetadata)
		outputWithMetadata.FromBytes(outputWithMetadataBytes)
		outputWithMetadata.SetID(*outputID)
		callback(outputWithMetadata)
		return true
	})
}

func (s *LedgerStateDiffs) delete(store kvstore.KVStore, outputID utxo.OutputID) (err error) {
	if err := store.Delete(lo.PanicOnErr(outputID.Bytes())); err != nil {
		return errors.Errorf("failed to delete output %s: %w", outputID, err)
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
