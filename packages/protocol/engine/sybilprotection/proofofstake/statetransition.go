package proofofstake

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/state"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type BatchedTransition struct {
	weights       *sybilprotection.Weights
	weightUpdates *sybilprotection.WeightUpdates
}

func NewBatchedTransition(weights *sybilprotection.Weights, targetEpoch epoch.Index) *BatchedTransition {
	return &BatchedTransition{
		weights:       weights,
		weightUpdates: sybilprotection.NewWeightUpdates(targetEpoch),
	}
}

func (m *BatchedTransition) Apply() (ctx context.Context) {
	m.weights.ApplyUpdates(m.weightUpdates)

	return nil
}

func (m *BatchedTransition) ProcessCreatedOutput(createdOutput *models.OutputWithMetadata) {
	if iotaBalance, exists := createdOutput.IOTABalance(); exists {
		m.weightUpdates.ApplyDiff(createdOutput.ConsensusManaPledgeID(), int64(iotaBalance))
	}
}

func (m *BatchedTransition) ProcessSpentOutput(spentOutput *models.OutputWithMetadata) {
	if iotaBalance, exists := spentOutput.IOTABalance(); exists {
		m.weightUpdates.ApplyDiff(spentOutput.ConsensusManaPledgeID(), -int64(iotaBalance))
	}
}

// code contract (make sure the type implements all required methods)
var _ state.BatchedTransition = &BatchedTransition{}
