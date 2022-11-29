package ledgerstate

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type DiffConsumer interface {
	Begin(committedEpoch epoch.Index)
	Commit() (ctx context.Context)
	ApplyCreatedOutput(output *OutputWithMetadata) (err error)
	ApplySpentOutput(output *OutputWithMetadata) (err error)
	RollbackCreatedOutput(output *OutputWithMetadata) (err error)
	RollbackSpentOutput(output *OutputWithMetadata) (err error)
	LastCommittedEpoch() epoch.Index
}
