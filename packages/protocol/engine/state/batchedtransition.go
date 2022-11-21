package state

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type BatchedTransition interface {
	ProcessCreatedOutput(output *models.OutputWithMetadata)
	ProcessSpentOutput(output *models.OutputWithMetadata)
	Apply() (ctx context.Context)
}
