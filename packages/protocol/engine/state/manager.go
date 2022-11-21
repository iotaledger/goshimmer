package state

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type BatchedStateTransitions []BatchedTransition

func (b BatchedStateTransitions) ProcessCreatedOutput(metadata *models.OutputWithMetadata) {
	for _, stateTransition := range b {
		stateTransition.ProcessCreatedOutput(metadata)
	}
}

func (b BatchedStateTransitions) ProcessSpentOutput(metadata *models.OutputWithMetadata) {
	for _, stateTransition := range b {
		stateTransition.ProcessSpentOutput(metadata)
	}
}

func (b BatchedStateTransitions) Apply() {
	for _, stateTransition := range b {
		stateTransition.Apply()
	}
}

type Manager struct {
	storage   *storage.Storage
	consumers []UpdateConsumer
	mutex     sync.RWMutex
}

func (s *Manager) RegisterStateUpdateConsumer(consumer UpdateConsumer) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.consumers = append(s.consumers, consumer)
}

func (s *Manager) CreateBatchedStateTransitions(targetEpoch epoch.Index) (batchedStateTransitions BatchedStateTransitions) {
	batchedStateTransitions = make([]BatchedTransition, len(s.consumers))
	for i, consumer := range s.consumers {
		batchedStateTransitions[i] = consumer.CreateBatchedStateTransition(targetEpoch)
	}

	return batchedStateTransitions
}

func (s *Manager) ApplyStateDiff(targetEpoch epoch.Index) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	batchedStateTransitions := make(BatchedStateTransitions, len(s.consumers))
	for i, consumer := range s.consumers {
		batchedStateTransitions[i] = consumer.CreateBatchedStateTransition(targetEpoch)
	}

	for _, createdOutput := range []*models.OutputWithMetadata{} {
		for i, pendingStateTransition := range batchedStateTransitions {
			switch {
			case s.consumers[i].LastConsumedEpoch() < targetEpoch:
				pendingStateTransition.ProcessCreatedOutput(createdOutput)
			case s.consumers[i].LastConsumedEpoch() > targetEpoch:
				pendingStateTransition.ProcessSpentOutput(createdOutput)
			}
		}
	}

	for _, spentOutput := range []*models.OutputWithMetadata{} {
		for i, pendingStateTransition := range batchedStateTransitions {
			switch {
			case s.consumers[i].LastConsumedEpoch() < targetEpoch:
				pendingStateTransition.ProcessSpentOutput(spentOutput)
			case s.consumers[i].LastConsumedEpoch() > targetEpoch:
				pendingStateTransition.ProcessCreatedOutput(spentOutput)
			}
		}
	}
}
