package dpos

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
)

func (s *SybilProtection) Begin(committedEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	s.setBatchedEpoch(committedEpoch)
	s.batchedWeightUpdates = sybilprotection.NewWeightUpdates(committedEpoch)

	// TODO: FIX current epoch shenanigans

	return
}

func (s *SybilProtection) ApplyCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	if s.batchedEpoch() == 0 {
		ApplyCreatedOutput(output, s.weights.Import)
	} else {
		ApplyCreatedOutput(output, s.batchedWeightUpdates.ApplyDiff)
	}

	return
}

func (s *SybilProtection) ApplySpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	ApplySpentOutput(output, s.batchedWeightUpdates.ApplyDiff)

	return
}

func (s *SybilProtection) RollbackCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return s.ApplySpentOutput(output)
}

func (s *SybilProtection) RollbackSpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return s.ApplyCreatedOutput(output)
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
	s.batchEpochMutex.RLock()
	defer s.batchEpochMutex.RUnlock()

	return s.batchEpochIndex
}

func (s *SybilProtection) setBatchedEpoch(index epoch.Index) {
	s.batchEpochMutex.Lock()
	defer s.batchEpochMutex.Unlock()

	if index != 0 && s.batchEpochIndex != 0 {
		panic("a batch is already in progress")
	}

	s.batchEpochIndex = index
}

var _ ledgerstate.UnspentOutputsConsumer = &SybilProtection{}
