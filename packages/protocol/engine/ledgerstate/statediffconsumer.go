package ledgerstate

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type DiffConsumer interface {
	ImportOutputs(outputs []*OutputWithMetadata)
	Begin(committedEpoch epoch.Index)
	Commit() (ctx context.Context)
	ApplyCreatedOutput(output *OutputWithMetadata)
	ApplySpentOutput(output *OutputWithMetadata)
	RollbackCreatedOutput(output *OutputWithMetadata)
	RollbackSpentOutput(output *OutputWithMetadata)
	LastCommittedEpoch() epoch.Index
}
