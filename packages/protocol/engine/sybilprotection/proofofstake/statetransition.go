package proofofstake

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type BufferedStateTransition struct {
	weights       *sybilprotection.Weights
	weightUpdates sybilprotection.WeightUpdates
}

func (m *BufferedStateTransition) Apply() (ctx context.Context) {
	m.weights.Apply(m.weightUpdates)

	return nil
}

func (m *BufferedStateTransition) ProcessCreatedOutput(createdOutput *models.OutputWithMetadata) {
	if iotaBalance, exists := createdOutput.IOTABalance(); exists {
		m.weightUpdates.ApplyDiff(createdOutput.ConsensusManaPledgeID(), int64(iotaBalance))
	}
}

func (m *BufferedStateTransition) ProcessSpentOutput(spentOutput *models.OutputWithMetadata) {
	if iotaBalance, exists := spentOutput.IOTABalance(); exists {
		m.weightUpdates.ApplyDiff(spentOutput.ConsensusManaPledgeID(), -int64(iotaBalance))
	}
}
