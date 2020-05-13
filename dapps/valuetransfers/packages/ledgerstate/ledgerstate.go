package ledgerstate

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
)

// LedgerState represents a struct, that allows us to read the balances from the UTXODAG by filtering the existing
// unspent Outputs depending on the liked branches.
type LedgerState struct {
	tangle *tangle.Tangle
}

// New is the constructor of the LedgerState. It creates a new instance with the given UTXODAG.
func New(tangle *tangle.Tangle) *LedgerState {
	return &LedgerState{
		tangle: tangle,
	}
}
