package ledgerstate

import (
	"io"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/pkg/errors"

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

func New(storageInstance *storage.Storage) (ledgerState *LedgerState) {
	ledgerState = &LedgerState{
		Initializable:  traits.NewInitializable(ledgerState.UnspentOutputs.TriggerInitialized, ledgerState.StateDiffs.TriggerInitialized),
		StateDiffs:     NewStateDiffs(storageInstance),
		UnspentOutputs: NewUnspentOutputs(storageInstance.UnspentOutputIDs, ledgerState.MemPool.Storage),
		storage:        storageInstance,
	}

	ledgerState.MemPool.Events.TransactionAccepted.Hook(event.NewClosure(ledgerState.onTransactionAccepted))
	ledgerState.MemPool.Events.TransactionInclusionUpdated.Hook(event.NewClosure(ledgerState.onTransactionInclusionUpdated))

	return
}

func (l *LedgerState) ApplyStateDiff(targetEpoch epoch.Index) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	currentEpoch, err := l.UnspentOutputs.Begin(targetEpoch)
	if err != nil {
		return errors.Errorf("failed to begin unspent outputs: %w", err)
	}

	if currentEpoch == targetEpoch {
		return
	}

	switch {
	case IsRollback(currentEpoch, targetEpoch):
		if err = l.StateDiffs.StreamSpentOutputs(currentEpoch, l.UnspentOutputs.RollbackSpentOutput); err != nil {
			return errors.Errorf("failed to apply created outputs: %w", err)
		}

		if err = l.StateDiffs.StreamCreatedOutputs(currentEpoch, l.UnspentOutputs.RollbackCreatedOutput); err != nil {
			return errors.Errorf("failed to apply spent outputs: %w", err)
		}
	default:
		if err = l.StateDiffs.StreamCreatedOutputs(targetEpoch, l.UnspentOutputs.ApplyCreatedOutput); err != nil {
			return errors.Errorf("failed to apply created outputs: %w", err)
		}

		if err = l.StateDiffs.StreamSpentOutputs(targetEpoch, l.UnspentOutputs.ApplySpentOutput); err != nil {
			return errors.Errorf("failed to apply spent outputs: %w", err)
		}
	}

	<-l.UnspentOutputs.Commit().Done()

	return
}

func (l *LedgerState) Import(reader io.ReadSeeker) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if err = l.UnspentOutputs.Import(reader); err != nil {
		return errors.Errorf("failed to import unspent outputs: %w", err)
	}

	importedStateDiffs, err := l.StateDiffs.Import(reader)
	if err != nil {
		return errors.Errorf("failed to import state diffs: %w", err)
	}

	for _, epochIndex := range importedStateDiffs {
		if err = l.ApplyStateDiff(epochIndex); err != nil {
			return errors.Errorf("failed to apply state diff %d: %w", epochIndex, err)
		}

		commitment, err := l.storage.Commitments.Load(epochIndex)
		if err != nil {
			return errors.Errorf("failed to load commitment for epoch %d: %w", epochIndex, err)
		}

		if err := l.storage.Settings.SetLatestCommitment(commitment); err != nil {
			return errors.Errorf("failed to set latest commitment: %w", err)
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

func (l *LedgerState) onTransactionAccepted(metadata *ledger.TransactionMetadata) {
	if err := l.StateDiffs.addAcceptedTransaction(metadata); err != nil {
		// TODO: handle error gracefully
		panic(err)
	}
}

func (l *LedgerState) onTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	if l.MemPool.ConflictDAG.ConfirmationState(event.TransactionMetadata.ConflictIDs()).IsAccepted() {
		l.StateDiffs.moveTransactionToOtherEpoch(event.TransactionMetadata, event.PreviousInclusionTime, event.InclusionTime)
	}
}
