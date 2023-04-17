package ledger

import (
	"io"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/module"
)

// Ledger is an engine module that provides access to the persistent ledger state.
type Ledger interface {
	// Events is a dictionary for Ledger related events.
	Events() *Events

	// MemPool returns the MemPool implementation used by this ledger.
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
