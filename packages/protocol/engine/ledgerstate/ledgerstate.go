package ledgerstate

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/models"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/pkg/errors"
)

type LedgerState struct {
	storage          *storage.Storage
	unspentOutputIDs *UnspentOutputIDs
	consumers        []DiffConsumer
	consumersMutex   sync.RWMutex
}

func New(storageInstance *storage.Storage) (ledgerState *LedgerState) {
	ledgerState = &LedgerState{
		storage:          storageInstance,
		unspentOutputIDs: NewUnspentOutputIDs(storageInstance.UnspentOutputIDs),
		consumers:        make([]DiffConsumer, 0),
	}

	ledgerState.RegisterConsumer(ledgerState.unspentOutputIDs)

	return
}

func (l *LedgerState) RegisterConsumer(consumer DiffConsumer) {
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
		l.ProcessConsumers(targetEpoch, output, DiffConsumer.ProcessCreatedOutput, DiffConsumer.ProcessSpentOutput)
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %s", targetEpoch, err)
	}

	if err = l.storage.LedgerStateDiffs.StreamSpentOutputs(targetEpoch, func(output *models.OutputWithMetadata) {
		l.ProcessConsumers(targetEpoch, output, DiffConsumer.ProcessSpentOutput, DiffConsumer.ProcessCreatedOutput)
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %s", targetEpoch, err)
	}

	for _, consumer := range l.consumers {
		consumer.Commit()
	}

	return nil
}

func (l *LedgerState) ProcessConsumers(targetEpoch epoch.Index, output *models.OutputWithMetadata, applyFunc, rollbackFunc func(DiffConsumer, *models.OutputWithMetadata)) {
	for _, consumer := range l.consumers {
		switch currentEpoch := consumer.LastCommittedEpoch(); {
		case IsApply(currentEpoch, targetEpoch):
			applyFunc(consumer, output)

		case IsRollback(currentEpoch, targetEpoch):
			rollbackFunc(consumer, output)
		}
	}
}

func (l *LedgerState) Root() types.Identifier {
	return l.unspentOutputIDs.Root()
}

// region utility functions ////////////////////////////////////////////////////////////////////////////////////////////

func IsApply(currentEpoch, targetEpoch epoch.Index) bool {
	return currentEpoch < targetEpoch
}

func IsRollback(currentEpoch, targetEpoch epoch.Index) bool {
	return currentEpoch > targetEpoch
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
