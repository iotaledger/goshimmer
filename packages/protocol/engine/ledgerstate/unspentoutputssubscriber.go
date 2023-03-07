package ledgerstate

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/hive.go/core/slot"
)

// UnspentOutputsSubscriber is an interface that allows to subscribe to changes in the unspent outputs.
type UnspentOutputsSubscriber interface {
	// ApplyCreatedOutput is called when an output is created.
	ApplyCreatedOutput(*ledger.OutputWithMetadata) error

	// ApplySpentOutput is called when an output is spent.
	ApplySpentOutput(*ledger.OutputWithMetadata) error

	// RollbackCreatedOutput is called when a created output is rolled back.
	RollbackCreatedOutput(*ledger.OutputWithMetadata) error

	// RollbackSpentOutput is called when a spent output is rolled back.
	RollbackSpentOutput(*ledger.OutputWithMetadata) error

	// BeginBatchedStateTransition is called when a batched state transition is started.
	BeginBatchedStateTransition(slot.Index) (currentSlot slot.Index, err error)

	// CommitBatchedStateTransition is called when a batched state transition is committed.
	CommitBatchedStateTransition() context.Context
}
