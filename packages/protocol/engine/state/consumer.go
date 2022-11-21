package state

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type Consumer interface {
	ImportOutput(output *models.OutputWithMetadata)
	BatchedTransition(targetEpoch epoch.Index) BatchedTransition
	LastConsumedEpoch() epoch.Index
}
