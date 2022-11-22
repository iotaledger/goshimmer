package dpos

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

	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		// TODO: WAIT FOR WRITE COMPLETE + CLEAN

		cancelFunc()
	}()

	return ctx
}

func (m *BatchedTransition) ProcessCreatedOutput(output *models.OutputWithMetadata) {
	ProcessCreatedOutput(output, m.weightUpdates.ApplyDiff)
}

func (m *BatchedTransition) ProcessSpentOutput(output *models.OutputWithMetadata) {
	ProcessSpentOutput(output, m.weightUpdates.ApplyDiff)
}

// code contract (make sure the type implements all required methods)
var _ state.BatchedTransition = &BatchedTransition{}
