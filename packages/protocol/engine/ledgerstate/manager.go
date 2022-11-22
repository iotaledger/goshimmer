package ledgerstate

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/models"
	"github.com/pkg/errors"
)

type LedgerState struct {
	storage          *storage.Storage
	unspentOutputIDs *UnspentOutputIDs
	consumers        []StateDiffConsumer
	consumersMutex   sync.RWMutex
}

func New(storageInstance *storage.Storage) *LedgerState {
	return &LedgerState{
		storage:          storageInstance,
		unspentOutputIDs: NewUnspentOutputIDs(storageInstance.UnspentOutputIDs),
		consumers:        make([]StateDiffConsumer, 0),
	}
}

func (l *LedgerState) RegisterConsumer(consumer StateDiffConsumer) {
	l.consumersMutex.Lock()
	defer l.consumersMutex.Unlock()

	l.consumers = append(l.consumers, consumer)
}

func (l *LedgerState) ImportOutputs(outputs []*models.OutputWithMetadata) {
	l.consumersMutex.RLock()
	defer l.consumersMutex.RUnlock()

	for _, output := range outputs {
		for _, consumer := range l.consumers {
			consumer.ProcessCreatedOutput(output)
		}
	}
}

func (l *LedgerState) ApplyStateDiff(targetEpoch epoch.Index) (err error) {
	l.consumersMutex.RLock()
	defer l.consumersMutex.RUnlock()

	for _, consumer := range l.consumers {
		consumer.Begin(targetEpoch)
	}

	if err = l.storage.LedgerStateDiffs.StreamCreatedOutputs(targetEpoch, func(output *models.OutputWithMetadata) {
		switch currentEpoch := l.storage.Settings.LatestCommitment().Index(); {
		case IsApply(currentEpoch, targetEpoch):
			l.unspentOutputIDs.Add(output.ID())
		case IsRollback(currentEpoch, targetEpoch):
			l.unspentOutputIDs.Delete(output.ID())
		}

		l.ProcessConsumers(targetEpoch, output, StateDiffConsumer.ProcessCreatedOutput, StateDiffConsumer.ProcessSpentOutput)
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %w", targetEpoch, err)
	}

	if err = l.storage.LedgerStateDiffs.StreamSpentOutputs(targetEpoch, func(output *models.OutputWithMetadata) {
		switch currentEpoch := l.storage.Settings.LatestCommitment().Index(); {
		case IsApply(currentEpoch, targetEpoch):
			l.unspentOutputIDs.Delete(output.ID())
		case IsRollback(currentEpoch, targetEpoch):
			l.unspentOutputIDs.Add(output.ID())
		}

		l.ProcessConsumers(targetEpoch, output, StateDiffConsumer.ProcessSpentOutput, StateDiffConsumer.ProcessCreatedOutput)
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %w", targetEpoch, err)
	}

	return nil
}

func (l *LedgerState) ProcessConsumers(targetEpoch epoch.Index, output *models.OutputWithMetadata, applyFunc, rollbackFunc func(StateDiffConsumer, *models.OutputWithMetadata)) {
	for _, consumer := range l.consumers {
		switch currentEpoch := consumer.LastCommittedEpoch(); {
		case IsApply(currentEpoch, targetEpoch):
			applyFunc(consumer, output)

		case IsRollback(currentEpoch, targetEpoch):
			rollbackFunc(consumer, output)
		}
	}
}

// region utility functions ////////////////////////////////////////////////////////////////////////////////////////////

func IsApply(currentEpoch, targetEpoch epoch.Index) bool {
	return currentEpoch < targetEpoch
}

func IsRollback(currentEpoch, targetEpoch epoch.Index) bool {
	return currentEpoch > targetEpoch
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
