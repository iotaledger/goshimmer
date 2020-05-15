package ledgerstate

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/utxodag"
)

// LedgerState represents a struct, that allows us to read the balances from the UTXODAG by filtering the existing
// unspent Outputs depending on the liked branches.
type LedgerState struct {
	utxoDAG *utxodag.UTXODAG
}

// New is the constructor of the LedgerState. It creates a new instance with the given UTXODAG.
func New(utxoDAG *utxodag.UTXODAG) *LedgerState {
	return &LedgerState{
		utxoDAG: utxoDAG,
	}
}
