package wallet

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Output is a wallet specific representation of an output in the IOTA network.
type Output struct {
	Address        ledgerstate.Address
	TransactionID  ledgerstate.TransactionID
	Balances       *ledgerstate.ColoredBalances
	InclusionState InclusionState
}

// InclusionState is a container for the different flags of an output that define if it was accepted in the network.
type InclusionState struct {
	Liked       bool
	Confirmed   bool
	Rejected    bool
	Conflicting bool
	Spent       bool
}
