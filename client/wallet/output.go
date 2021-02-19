package wallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/stringify"
)

// Output is a wallet specific representation of an output in the IOTA network.
type Output struct {
	Address        address.Address
	OutputID       ledgerstate.OutputID
	Balances       *ledgerstate.ColoredBalances
	InclusionState InclusionState
}

// String returns a human-readable representation of the Output.
func (o *Output) String() string {
	return stringify.Struct("Output",
		stringify.StructField("Address", o.Address),
		stringify.StructField("OutputID", o.OutputID),
		stringify.StructField("Balances", o.Balances),
		stringify.StructField("InclusionState", o.InclusionState),
	)
}

// InclusionState is a container for the different flags of an output that define if it was accepted in the network.
type InclusionState struct {
	Liked       bool
	Confirmed   bool
	Rejected    bool
	Conflicting bool
	Spent       bool
}

// String returns a human-readable representation of the InclusionState.
func (i InclusionState) String() string {
	return stringify.Struct("InclusionState",
		stringify.StructField("Liked", i.Liked),
		stringify.StructField("Confirmed", i.Confirmed),
		stringify.StructField("Rejected", i.Rejected),
		stringify.StructField("Conflicting", i.Conflicting),
		stringify.StructField("Spent", i.Spent),
	)
}
