package ledgerstate

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type StateDiffConsumer interface {
	Begin(committedEpoch epoch.Index)
	Commit() (ctx context.Context)
	ProcessCreatedOutput(output *models.OutputWithMetadata)
	ProcessSpentOutput(output *models.OutputWithMetadata)
	LastCommittedEpoch() epoch.Index
}
