package ledgerstate

import (
	"context"
	"io"

	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/ads"
	"github.com/iotaledger/hive.go/core/slot"
)

type LedgerState interface {
	UnspentOutputs() UnspentOutputs

	StateDiffs() StateDiffs

	// ApplyStateDiff applies the state diff of the given slot to the ledger state.
	ApplyStateDiff(index slot.Index) (err error)

	Import(reader io.ReadSeeker) (err error)
	Export(writer io.WriteSeeker, targetSlot slot.Index) (err error)

	module.Interface
}

type UnspentOutputs interface {
	IDs() *ads.Set[utxo.OutputID, *utxo.OutputID]

	Subscribe(consumer UnspentOutputsConsumer)
	Unsubscribe(consumer UnspentOutputsConsumer)

	ApplyCreatedOutput(output *ledger.OutputWithMetadata) (err error)

	traits.BatchCommittable
	module.Interface
}

type UnspentOutputsConsumer interface {
	ApplyCreatedOutput(output *ledger.OutputWithMetadata) (err error)
	ApplySpentOutput(output *ledger.OutputWithMetadata) (err error)
	RollbackCreatedOutput(output *ledger.OutputWithMetadata) (err error)
	RollbackSpentOutput(output *ledger.OutputWithMetadata) (err error)
	BeginBatchedStateTransition(targetSlot slot.Index) (currentSlot slot.Index, err error)
	CommitBatchedStateTransition() (ctx context.Context)
}

type StateDiffs interface {
	StreamCreatedOutputs(index slot.Index, callback func(*ledger.OutputWithMetadata) error) (err error)

	StreamSpentOutputs(index slot.Index, callback func(*ledger.OutputWithMetadata) error) (err error)
}
