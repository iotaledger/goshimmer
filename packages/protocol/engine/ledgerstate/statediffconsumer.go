package ledgerstate

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type DiffConsumer interface {
	ImportOutputs(outputs []*OutputWithMetadata)
	Begin(committedEpoch epoch.Index)
	Commit() (ctx context.Context)
	ProcessCreatedOutput(output *OutputWithMetadata)
	ProcessSpentOutput(output *OutputWithMetadata)
	LastCommittedEpoch() epoch.Index
}
