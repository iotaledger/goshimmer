package ledgerstate

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type DiffConsumer interface {
	ImportOutputs(outputs []*models.OutputWithMetadata)
	Begin(committedEpoch epoch.Index)
	Commit() (ctx context.Context)
	ProcessCreatedOutput(output *models.OutputWithMetadata)
	ProcessSpentOutput(output *models.OutputWithMetadata)
	LastCommittedEpoch() epoch.Index
}
