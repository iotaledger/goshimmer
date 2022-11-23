package ledgerstate

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

const (
	spentOutputsPrefix byte = iota
	createdOutputsPrefix
)

// region StateDiffs ///////////////////////////////////////////////////////////////////////////////////////////////////

type StateDiffs struct {
	Storage func(index epoch.Index) kvstore.KVStore
	ledger  *ledger.Ledger
}

func NewStateDiffs(storage func(index epoch.Index) kvstore.KVStore) (newLedgerStateDiffs *StateDiffs) {
	return &StateDiffs{
		Storage: storage,
	}
}

func (s *StateDiffs) StoreSpentOutput(outputWithMetadata *OutputWithMetadata) (err error) {
	if spentStorage, spentStorageErr := s.Storage(outputWithMetadata.Index()).WithExtendedRealm([]byte{spentOutputsPrefix}); spentStorageErr != nil {
		return errors.Errorf("failed to retrieve spent storage: %w", spentStorageErr)
	} else {
		return spentStorage.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), lo.PanicOnErr(outputWithMetadata.Bytes()))
	}
}

func (s *StateDiffs) StoreCreatedOutput(outputWithMetadata *OutputWithMetadata) (err error) {
	if createdStorage, createdStorageErr := s.Storage(outputWithMetadata.Index()).WithExtendedRealm([]byte{createdOutputsPrefix}); createdStorageErr != nil {
		return errors.Errorf("failed to retrieve created storage: %w", createdStorageErr)
	} else {
		return createdStorage.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), lo.PanicOnErr(outputWithMetadata.Bytes()))
	}
}

func (s *StateDiffs) LoadSpentOutput(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	store, err := s.Storage(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return nil, errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return s.get(store, outputID)
}

func (s *StateDiffs) LoadCreatedOutput(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	store, err := s.Storage(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return nil, errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.get(store, outputID)
}

func (s *StateDiffs) DeleteSpentOutput(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := s.Storage(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	return s.delete(store, outputID)
}

func (s *StateDiffs) DeleteCreatedOutput(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := s.Storage(index).WithExtendedRealm([]byte{createdOutputsPrefix})
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

func (s *StateDiffs) StreamSpentOutputs(index epoch.Index, callback func(*OutputWithMetadata)) (err error) {
	store, err := s.Storage(index).WithExtendedRealm([]byte{spentOutputsPrefix})
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	s.stream(store, callback)

	return
}

func (s *StateDiffs) StreamCreatedOutputs(index epoch.Index, callback func(*OutputWithMetadata)) (err error) {
	store, err := s.Storage(index).WithExtendedRealm([]byte{createdOutputsPrefix})
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	s.stream(store, callback)

	return
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

func (s *StateDiffs) stream(store kvstore.KVStore, callback func(*OutputWithMetadata)) {
	store.Iterate([]byte{}, func(idBytes kvstore.Key, outputWithMetadataBytes kvstore.Value) bool {
		outputID := new(utxo.OutputID)
		outputID.FromBytes(idBytes)
		outputWithMetadata := new(OutputWithMetadata)
		outputWithMetadata.FromBytes(outputWithMetadataBytes)
		outputWithMetadata.SetID(*outputID)
		callback(outputWithMetadata)
		return true
	})
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
