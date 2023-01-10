package dpos

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

func (s *SybilProtection) Begin(committedEpoch epoch.Index) {
	s.setBatchedEpoch(committedEpoch)
	s.batchedWeightUpdates = sybilprotection.NewWeightUpdates(committedEpoch)
}

func (s *SybilProtection) ImportOutputs(outputs []*models.OutputWithMetadata) {
	for _, output := range outputs {
		ProcessCreatedOutput(output, s.weights.Import)
	}
}

func (s *SybilProtection) ProcessCreatedOutput(output *models.OutputWithMetadata) {
	if s.batchedEpoch() == 0 {
		ProcessCreatedOutput(output, s.weights.Import)
	} else {
		ProcessCreatedOutput(output, s.batchedWeightUpdates.ApplyDiff)
	}
}

func (s *SybilProtection) ProcessSpentOutput(output *models.OutputWithMetadata) {
	ProcessSpentOutput(output, s.batchedWeightUpdates.ApplyDiff)
}

func (s *SybilProtection) Commit() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())
	go func() {
		// TODO: WAIT FOR WRITE COMPLETE + CLEAN
		s.weights.ApplyUpdates(s.batchedWeightUpdates)

		s.SetLastCommittedEpoch(s.batchedEpoch())
		s.setBatchedEpoch(0)

		done()
	}()

	return ctx
}

func (s *SybilProtection) LastCommittedEpoch() epoch.Index {
	s.lastCommittedEpochMutex.RLock()
	defer s.lastCommittedEpochMutex.RUnlock()

	return s.lastCommittedEpoch
}

func (s *SybilProtection) SetLastCommittedEpoch(index epoch.Index) {
	s.lastCommittedEpochMutex.Lock()
	defer s.lastCommittedEpochMutex.Unlock()

	s.lastCommittedEpoch = index
}

func (s *SybilProtection) batchedEpoch() epoch.Index {
	s.batchedEpochMutex.RLock()
	defer s.batchedEpochMutex.RUnlock()

	return s.batchedEpochIndex
}

func (s *SybilProtection) setBatchedEpoch(index epoch.Index) {
	s.batchedEpochMutex.Lock()
	defer s.batchedEpochMutex.Unlock()

	if index != 0 && s.batchedEpochIndex != 0 {
		panic("a batch is already in progress")
	}

	s.batchedEpochIndex = index
}

var _ ledgerstate.DiffConsumer = &SybilProtection{}
