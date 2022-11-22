package state

import (
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type Manager struct {
	storage        *storage.Storage
	consumers      []Consumer
	consumersMutex sync.RWMutex
}

func NewManager(storageInstance *storage.Storage) *Manager {
	return &Manager{
		storage:   storageInstance,
		consumers: make([]Consumer, 0),
	}
}

func (s *Manager) RegisterConsumer(consumer Consumer) {
	s.consumersMutex.Lock()
	defer s.consumersMutex.Unlock()

	s.consumers = append(s.consumers, consumer)
}

func (s *Manager) ApplyStateDiff(targetEpoch epoch.Index) (err error) {
	s.consumersMutex.RLock()
	defer s.consumersMutex.RUnlock()

	batchedTransitions := make([]BatchedTransition, len(s.consumers))
	for i, consumer := range s.consumers {
		batchedTransitions[i] = consumer.BatchedTransition(targetEpoch)
	}

	if err = s.storage.LedgerStateDiffs.StreamCreatedOutputs(targetEpoch, func(output *models.OutputWithMetadata) {
		for i, batchedTransition := range batchedTransitions {
			switch {
			case s.consumers[i].LastConsumedEpoch() < targetEpoch:
				batchedTransition.ProcessCreatedOutput(output)
			case s.consumers[i].LastConsumedEpoch() > targetEpoch:
				batchedTransition.ProcessSpentOutput(output)
			}
		}
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %w", targetEpoch, err)
	}

	if err = s.storage.LedgerStateDiffs.StreamSpentOutputs(targetEpoch, func(output *models.OutputWithMetadata) {
		for i, pendingStateTransition := range batchedTransitions {
			switch {
			case s.consumers[i].LastConsumedEpoch() < targetEpoch:
				pendingStateTransition.ProcessSpentOutput(output)
			case s.consumers[i].LastConsumedEpoch() > targetEpoch:
				pendingStateTransition.ProcessCreatedOutput(output)
			}
		}
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %w", targetEpoch, err)
	}

	return nil
}

func (s *Manager) ImportOutputs(outputs []*models.OutputWithMetadata) {
	s.consumersMutex.RLock()
	defer s.consumersMutex.RUnlock()

	for _, output := range outputs {
		for _, consumer := range s.consumers {
			consumer.ImportOutput(output)
		}
	}
}
