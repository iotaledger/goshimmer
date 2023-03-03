package ledgerstate

import (
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
)

// LedgerState represents the state of the ledger.
type LedgerState struct {
	MemPool        *ledger.Ledger
	UnspentOutputs *UnspentOutputs
	StateDiffs     *StateDiffs

	storage *storage.Storage
	mutex   sync.RWMutex

	module.Module
}

// New creates a new ledger state.
func New(storageInstance *storage.Storage, memPool *ledger.Ledger) (ledgerState *LedgerState) {
	unspentOutputs := NewUnspentOutputs(storageInstance.UnspentOutputIDs, memPool)

	ledgerState = &LedgerState{
		MemPool:        memPool,
		UnspentOutputs: unspentOutputs,
		StateDiffs:     NewStateDiffs(storageInstance, memPool),

		storage: storageInstance,
	}

	ledgerState.HookInitialized(unspentOutputs.TriggerInitialized)

	ledgerState.MemPool.Events.TransactionAccepted.Hook(ledgerState.onTransactionAccepted)
	ledgerState.MemPool.Events.TransactionInclusionUpdated.Hook(ledgerState.onTransactionInclusionUpdated)

	return
}

// ApplyStateDiff applies the state diff of the given slot to the ledger state.
func (l *LedgerState) ApplyStateDiff(index slot.Index) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lastCommittedSlot, err := l.UnspentOutputs.Begin(index)
	if err != nil {
		return errors.Wrap(err, "failed to begin unspent outputs")
	} else if lastCommittedSlot == index {
		return
	}

	if err = l.StateDiffs.StreamCreatedOutputs(index, l.UnspentOutputs.ApplyCreatedOutput); err != nil {
		return errors.Wrap(err, "failed to apply created outputs")
	}

	if err = l.StateDiffs.StreamSpentOutputs(index, l.UnspentOutputs.ApplySpentOutput); err != nil {
		return errors.Wrap(err, "failed to apply spent outputs")
	}

	<-l.UnspentOutputs.Commit().Done()

	return
}

// Import imports the ledger state from the given reader.
func (l *LedgerState) Import(reader io.ReadSeeker) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if err = l.UnspentOutputs.Import(reader, l.storage.Settings.LatestCommitment().Index()); err != nil {
		return errors.Wrap(err, "failed to import unspent outputs")
	}

	importedStateDiffs, err := l.StateDiffs.Import(reader)
	if err != nil {
		return errors.Wrap(err, "failed to import state diffs")
	}

	// Apply state diffs backwards.
	if len(importedStateDiffs) != 0 {
		var stateDiffSlot slot.Index
		for _, stateDiffSlot = range importedStateDiffs {
			if err = l.rollbackStateDiff(stateDiffSlot); err != nil {
				return errors.Wrapf(err, "failed to apply state diff %d", stateDiffSlot)
			}

			if err = l.StateDiffs.Delete(stateDiffSlot); err != nil {
				return errors.Wrapf(err, "failed to delete state diff %d", stateDiffSlot)
			}
		}
		stateDiffSlot-- // we rolled back slot n to get to slot n-1

		targetSlotCommitment, errLoad := l.storage.Commitments.Load(stateDiffSlot)
		if errLoad != nil {
			return errors.Wrapf(errLoad, "failed to load commitment for target slot %d", stateDiffSlot)
		}

		if err = l.storage.Settings.SetLatestCommitment(targetSlotCommitment); err != nil {
			return errors.Wrap(err, "failed to set latest commitment")
		}

		if err = l.storage.Settings.SetLatestStateMutationSlot(stateDiffSlot); err != nil {
			return errors.Wrap(err, "failed to set latest state mutation slot")
		}

		if err = l.storage.Settings.SetLatestConfirmedSlot(stateDiffSlot); err != nil {
			return errors.Wrap(err, "failed to set latest confirmed slot")
		}
	}

	l.TriggerInitialized()

	return
}

// Export exports the ledger state to the given writer.
func (l *LedgerState) Export(writer io.WriteSeeker, targetSlot slot.Index) (err error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	if err = l.UnspentOutputs.Export(writer); err != nil {
		return errors.Wrap(err, "failed to export unspent outputs")
	}

	if err = l.StateDiffs.Export(writer, targetSlot); err != nil {
		return errors.Wrap(err, "failed to export state diffs")
	}

	return
}

// rollbackStateDiff rolls back the named stateDiff index to get to the previous slot.
func (l *LedgerState) rollbackStateDiff(index slot.Index) (err error) {
	targetSlot := index - 1
	lastCommittedSlot, err := l.UnspentOutputs.Begin(targetSlot)
	if err != nil {
		return errors.Wrap(err, "failed to begin unspent outputs")
	} else if lastCommittedSlot == targetSlot {
		return
	}

	if err = l.StateDiffs.StreamSpentOutputs(lastCommittedSlot, l.UnspentOutputs.RollbackSpentOutput); err != nil {
		return errors.Wrap(err, "failed to apply created outputs")
	}

	if err = l.StateDiffs.StreamCreatedOutputs(lastCommittedSlot, l.UnspentOutputs.RollbackCreatedOutput); err != nil {
		return errors.Wrap(err, "failed to apply spent outputs")
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
		l.StateDiffs.moveTransactionToOtherSlot(inclusionUpdatedEvent.TransactionMetadata, inclusionUpdatedEvent.PreviousInclusionSlot, inclusionUpdatedEvent.InclusionSlot)
	}
}
