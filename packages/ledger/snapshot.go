package ledger

import (
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// Snapshot is a snapshot of the ledger state.
type Snapshot struct {
	Outputs []utxo.Output
}

// NewSnapshot creates a new snapshot.
func NewSnapshot(outputs ...utxo.Output) (new *Snapshot) {
	return &Snapshot{
		Outputs: outputs,
	}
}
