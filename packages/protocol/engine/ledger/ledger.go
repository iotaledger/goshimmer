package ledger

import (
	"io"

	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/hive.go/core/slot"
)

// Ledger is an engine module that provides access to the persistent ledger state.
type Ledger interface {
	MemPool() mempool.MemPool

	// UnspentOutputs returns the unspent outputs of the ledger state.
	UnspentOutputs() UnspentOutputs

	// StateDiffs returns the state diffs of the ledger state.
	StateDiffs() StateDiffs

	// ApplyStateDiff applies the state diff of the given slot index.
	ApplyStateDiff(slot.Index) error

	// Import imports the ledger state from the given reader.
	Import(io.ReadSeeker) error

	// Export exports the ledger state to the given writer.
	Export(io.WriteSeeker, slot.Index) error

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
