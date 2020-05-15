package tangle

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
)

// LedgerState represents a struct, that allows us to read the balances from the UTXODAG by filtering the existing
// unspent Outputs depending on the liked branches.
type LedgerState struct {
	tangle *Tangle
}

// NewLedgerState is the constructor of the LedgerState. It creates a new instance with the given UTXODAG.
func NewLedgerState(tangle *Tangle) *LedgerState {
	return &LedgerState{
		tangle: tangle,
	}
}

// Balances returns a map containing the balances of the different colors that are unspent on a certain address.
func (ledgerState *LedgerState) Balances(address address.Address) (coloredBalances map[balance.Color]int64) {
	coloredBalances = make(map[balance.Color]int64)

	ledgerState.tangle.OutputsOnAddress(address).Consume(func(output *Output) {
		if output.ConsumerCount() == 0 {
			for _, coloredBalance := range output.Balances() {
				coloredBalances[coloredBalance.Color()] += coloredBalance.Value()
			}
		}
	})

	return
}
