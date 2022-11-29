package ledgerstate

import (
	"io"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type LedgerState struct {
	MemPool        *ledger.Ledger
	StateDiffs     *StateDiffs
	UnspentOutputs *UnspentOutputs
	storage        *storage.Storage
}

func New(storageInstance *storage.Storage) (ledgerState *LedgerState) {
	ledgerState = &LedgerState{
		StateDiffs:     NewStateDiffs(storageInstance),
		UnspentOutputs: NewUnspentOutputs(storageInstance.UnspentOutputIDs, ledgerState.MemPool.Storage),
		storage:        storageInstance,
	}

	ledgerState.MemPool.Events.TransactionAccepted.Hook(event.NewClosure(ledgerState.onTransactionAccepted))
	ledgerState.MemPool.Events.TransactionInclusionUpdated.Hook(event.NewClosure(ledgerState.onTransactionInclusionUpdated))

	return
}

func (l *LedgerState) ApplyStateDiff(targetEpoch epoch.Index) (err error) {
	direction, err := l.UnspentOutputs.Begin(targetEpoch)
	if err != nil {
		return err
	}

	switch {
	case direction > 0:
		if err = l.StateDiffs.StreamCreatedOutputs(targetEpoch, l.UnspentOutputs.ApplyCreatedOutput); err != nil {
			return errors.Errorf("failed to apply created outputs: %w", err)
		} else if err = l.StateDiffs.StreamSpentOutputs(targetEpoch, l.UnspentOutputs.ApplySpentOutput); err != nil {
			return errors.Errorf("failed to apply spent outputs: %w", err)
		}
	case direction < 0:
		if err = l.StateDiffs.StreamSpentOutputs(targetEpoch, l.UnspentOutputs.RollbackSpentOutput); err != nil {
			return errors.Errorf("failed to apply created outputs: %w", err)
		} else if err = l.StateDiffs.StreamCreatedOutputs(targetEpoch, l.UnspentOutputs.RollbackCreatedOutput); err != nil {
			return errors.Errorf("failed to apply spent outputs: %w", err)
		}
	}

	l.UnspentOutputs.Commit()

	return nil
}

func (l *LedgerState) Import(reader io.ReadSeeker) (err error) {
	if err = l.UnspentOutputs.Import(reader); err != nil {
		return errors.Errorf("failed to import unspent outputs: %w", err)
	} else if importedStateDiffs, err := l.StateDiffs.Import(reader); err != nil {
		return errors.Errorf("failed to import state diffs: %w", err)
	} else {
		for _, epochIndex := range importedStateDiffs {
			if err = l.ApplyStateDiff(epochIndex); err != nil {
				return errors.Errorf("failed to apply state diff %d: %w", epochIndex, err)
			}
		}
	}

	return
}

func (l *LedgerState) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if err = l.UnspentOutputs.Export(writer, targetEpoch); err != nil {
		return errors.Errorf("failed to export unspent outputs: %w", err)
	} else if err = l.StateDiffs.Export(writer, targetEpoch); err != nil {
		return errors.Errorf("failed to export state diffs: %w", err)
	}

	return nil
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
