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

type LedgerState struct {
	MemPool        *ledger.Ledger
	StateDiffs     *StateDiffs
	UnspentOutputs *UnspentOutputs
	storage        *storage.Storage
	mutex          sync.RWMutex

	traits.Initializable
}

func New(storageInstance *storage.Storage, memPool *ledger.Ledger) (ledgerState *LedgerState) {
	ledgerState = &LedgerState{
		MemPool:        memPool,
		StateDiffs:     NewStateDiffs(storageInstance, memPool),
		UnspentOutputs: NewUnspentOutputs(storageInstance.UnspentOutputIDs, memPool),
		storage:        storageInstance,
	}

	ledgerState.Initializable = traits.NewInitializable(ledgerState.UnspentOutputs.TriggerInitialized)

	ledgerState.MemPool.Events.TransactionAccepted.Hook(event.NewClosure(ledgerState.onTransactionAccepted))
	ledgerState.MemPool.Events.TransactionInclusionUpdated.Hook(event.NewClosure(ledgerState.onTransactionInclusionUpdated))

	return
}

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

func (l *LedgerState) ApplyStateDiff(targetEpoch epoch.Index) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	return l.applyStateDiff(targetEpoch)
}

// applyStateDiff applies the stateDiff for the given index to get to that epoch.
func (l *LedgerState) applyStateDiff(index epoch.Index) (err error) {
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

func (l *LedgerState) onTransactionAccepted(metadata *ledger.TransactionMetadata) {
	if err := l.StateDiffs.addAcceptedTransaction(metadata); err != nil {
		// TODO: handle error gracefully
		panic(err)
	}
}

func (l *LedgerState) onTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	if l.MemPool.ConflictDAG.ConfirmationState(event.TransactionMetadata.ConflictIDs()).IsAccepted() {
		l.StateDiffs.moveTransactionToOtherEpoch(event.TransactionMetadata, event.PreviousInclusionEpoch, event.InclusionEpoch)
	}
}
