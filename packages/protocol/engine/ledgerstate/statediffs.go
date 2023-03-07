package ledgerstate

import (
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/hive.go/core/slot"
)

// StateDiffs is a submodule that provides access to the state diffs of the ledger state.
type StateDiffs interface {
	// StreamCreatedOutputs streams the created outputs of the given slot index.
	StreamCreatedOutputs(slot.Index, func(*ledger.OutputWithMetadata) error) error

	// StreamSpentOutputs streams the spent outputs of the given slot index.
	StreamSpentOutputs(slot.Index, func(*ledger.OutputWithMetadata) error) error
}
