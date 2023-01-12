package ledgerstate

import (
	"io"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// LedgerState represents the state of the ledger.
type LedgerState struct {
	MemPool        *ledger.Ledger
	UnspentOutputs *UnspentOutputs
	StateDiffs     *StateDiffs

	storage *storage.Storage
	mutex   sync.RWMutex

	traits.Initializable
}

// New creates a new ledger state.
func New(storageInstance *storage.Storage, memPool *ledger.Ledger) (ledgerState *LedgerState) {
	unspentOutputs := NewUnspentOutputs(storageInstance.UnspentOutputIDs, memPool)

	ledgerState = &LedgerState{
		MemPool:        memPool,
		UnspentOutputs: unspentOutputs,
		StateDiffs:     NewStateDiffs(storageInstance, memPool),
		Initializable:  traits.NewInitializable(unspentOutputs.TriggerInitialized),

		storage: storageInstance,
	}

	ledgerState.MemPool.Events.TransactionAccepted.Hook(event.NewClosure(ledgerState.onTransactionAccepted))
	ledgerState.MemPool.Events.TransactionInclusionUpdated.Hook(event.NewClosure(ledgerState.onTransactionInclusionUpdated))

	return
}

// ApplyStateDiff applies the state diff of the given epoch to the ledger state.
func (l *LedgerState) ApplyStateDiff(index epoch.Index) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lastCommittedEpoch, err := l.UnspentOutputs.Begin(index)
	if err != nil {
		return errors.Errorf("failed to begin unspent outputs: %w", err)
	} else if lastCommittedEpoch == index {
		return
	}

	if err = l.StateDiffs.StreamCreatedOutputs(index, l.UnspentOutputs.ApplyCreatedOutput); err != nil {
		return errors.Errorf("failed to apply created outputs: %w", err)
	}

	if err = l.StateDiffs.StreamSpentOutputs(index, l.UnspentOutputs.ApplySpentOutput); err != nil {
		return errors.Errorf("failed to apply spent outputs: %w", err)
	}

	<-l.UnspentOutputs.Commit().Done()

	return
}

// Import imports the ledger state from the given reader.
func (l *LedgerState) Import(reader io.ReadSeeker) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if err = l.UnspentOutputs.Import(reader, l.storage.Settings.LatestCommitment().Index()); err != nil {
		return errors.Errorf("failed to import unspent outputs: %w", err)
	}

	importedStateDiffs, err := l.StateDiffs.Import(reader)
	if err != nil {
		return errors.Errorf("failed to import state diffs: %w", err)
	}

	// Apply state diffs backwards.
	if len(importedStateDiffs) != 0 {
		var stateDiffEpoch epoch.Index
		for _, stateDiffEpoch = range importedStateDiffs {
			if err = l.rollbackStateDiff(stateDiffEpoch); err != nil {
				return errors.Errorf("failed to apply state diff %d: %w", stateDiffEpoch, err)
			}

			if err = l.StateDiffs.Delete(stateDiffEpoch); err != nil {
				return errors.Errorf("failed to delete state diff %d: %w", stateDiffEpoch, err)
			}
		}
		stateDiffEpoch-- // we rolled back epoch n to get to epoch n-1

		targetEpochCommitment, errLoad := l.storage.Commitments.Load(stateDiffEpoch)
		if errLoad != nil {
			return errors.Errorf("failed to load commitment for target epoch %d: %w", stateDiffEpoch, errLoad)
		}

		if err = l.storage.Settings.SetLatestCommitment(targetEpochCommitment); err != nil {
			return errors.Errorf("failed to set latest commitment: %w", err)
		}

		if err = l.storage.Settings.SetLatestStateMutationEpoch(stateDiffEpoch); err != nil {
			return errors.Errorf("failed to set latest state mutation epoch: %w", err)
		}

		if err = l.storage.Settings.SetLatestConfirmedEpoch(stateDiffEpoch); err != nil {
			return errors.Errorf("failed to set latest confirmed epoch: %w", err)
		}
	}

	l.TriggerInitialized()

	return
}

// Export exports the ledger state to the given writer.
func (l *LedgerState) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	if err = l.UnspentOutputs.Export(writer); err != nil {
		return errors.Errorf("failed to export unspent outputs: %w", err)
	}

	if err = l.StateDiffs.Export(writer, targetEpoch); err != nil {
		return errors.Errorf("failed to export state diffs: %w", err)
	}

	return
}

// rollbackStateDiff rolls back the named stateDiff index to get to the previous epoch.
func (l *LedgerState) rollbackStateDiff(index epoch.Index) (err error) {
	targetEpoch := index - 1
	lastCommittedEpoch, err := l.UnspentOutputs.Begin(targetEpoch)
	if err != nil {
		return errors.Errorf("failed to begin unspent outputs: %w", err)
	} else if lastCommittedEpoch == targetEpoch {
		return
	}

	if err = l.StateDiffs.StreamSpentOutputs(lastCommittedEpoch, l.UnspentOutputs.RollbackSpentOutput); err != nil {
		return errors.Errorf("failed to apply created outputs: %w", err)
	}

	if err = l.StateDiffs.StreamCreatedOutputs(lastCommittedEpoch, l.UnspentOutputs.RollbackCreatedOutput); err != nil {
		return errors.Errorf("failed to apply spent outputs: %w", err)
	}

	<-l.UnspentOutputs.Commit().Done()

	return
}

// onTransactionAccepted is triggered when a transaction is accepted by the mempool.
func (l *LedgerState) onTransactionAccepted(transactionEvent *ledger.TransactionEvent) {
	if err := l.StateDiffs.addAcceptedTransaction(transactionEvent.Metadata); err != nil {
		// TODO: handle error gracefully
		panic(err)
	}
}

// onTransactionInclusionUpdated is triggered when a transaction inclusion state is updated.
func (l *LedgerState) onTransactionInclusionUpdated(inclusionUpdatedEvent *ledger.TransactionInclusionUpdatedEvent) {
	if l.MemPool.ConflictDAG.ConfirmationState(inclusionUpdatedEvent.TransactionMetadata.ConflictIDs()).IsAccepted() {
		l.StateDiffs.moveTransactionToOtherEpoch(inclusionUpdatedEvent.TransactionMetadata, inclusionUpdatedEvent.PreviousInclusionEpoch, inclusionUpdatedEvent.InclusionEpoch)
	}
}
