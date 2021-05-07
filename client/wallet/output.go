package wallet

import (
	"time"

	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output is a wallet specific representation of an output in the IOTA network.
type Output struct {
	Address        address.Address
	OutputID       ledgerstate.OutputID
	Balances       *ledgerstate.ColoredBalances
	InclusionState InclusionState
	Metadata       OutputMetadata
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region InclusionState ///////////////////////////////////////////////////////////////////////////////////////////////

// InclusionState is a container for the different flags of an output that define if it was accepted in the network.
type InclusionState struct {
	Liked       bool
	Confirmed   bool
	Rejected    bool
	Conflicting bool
	Spent       bool
}

// OutputMetadata is metadata about the output.
type OutputMetadata struct {
	// Timestamp is the timestamp of the tx that created the output.
	Timestamp time.Time
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsByID //////////////////////////////////////////////////////////////////////////////////////////////////

// OutputsByID is a collection of Outputs associated with their OutputID.
type OutputsByID map[ledgerstate.OutputID]*Output

// OutputsByAddressAndOutputID returns a collection of Outputs associated with their Address and OutputID.
func (o OutputsByID) OutputsByAddressAndOutputID() (outputsByAddressAndOutputID OutputsByAddressAndOutputID) {
	outputsByAddressAndOutputID = make(OutputsByAddressAndOutputID)
	for outputID, output := range o {
		outputsByAddress, exists := outputsByAddressAndOutputID[output.Address]
		if !exists {
			outputsByAddress = make(OutputsByID)
			outputsByAddressAndOutputID[output.Address] = outputsByAddress
		}

		outputsByAddress[outputID] = output
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsByAddressAndOutputID //////////////////////////////////////////////////////////////////////////////////

// OutputsByAddressAndOutputID is a collection of Outputs associated with their Address and OutputID.
type OutputsByAddressAndOutputID map[address.Address]map[ledgerstate.OutputID]*Output

// OutputsByID returns a collection of Outputs associated with their OutputID.
func (o OutputsByAddressAndOutputID) OutputsByID() (outputsByID OutputsByID) {
	outputsByID = make(OutputsByID)
	for _, outputs := range o {
		for outputID, output := range outputs {
			outputsByID[outputID] = output
		}
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
