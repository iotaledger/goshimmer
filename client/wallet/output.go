package wallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/stringify"
	"time"
)

// Output is a wallet specific representation of an output in the IOTA network.
type Output struct {
	Address        address.Address
	Object         ledgerstate.Output
	InclusionState InclusionState
	Metadata       OutputMetadata
}

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

type OutputsByAddressAndOutputID map[address.Address]map[ledgerstate.OutputID]*Output

func NewAddressToOutputs() OutputsByAddressAndOutputID {
	return make(map[address.Address]map[ledgerstate.OutputID]*Output)
}

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

func (o OutputsByAddressAndOutputID) ValueOutputsOnly() OutputsByAddressAndOutputID {
	result := NewAddressToOutputs()
	for addy, IDToOutputMap := range o {
		for outputID, output := range IDToOutputMap {
			switch output.Object.Type() {
			case ledgerstate.SigLockedSingleOutputType, ledgerstate.SigLockedColoredOutputType, ledgerstate.ExtendedLockedOutputType:
				if _, addressExists := result[addy]; !addressExists {
					result[addy] = make(map[ledgerstate.OutputID]*Output)
				}
				result[addy][outputID] = output
			}
		}
	}
	return result
}

// ConditionalOutputsOnly return ExtendedLockedOutputs that are currently conditionally owned by the wallet.
func (o OutputsByAddressAndOutputID) ConditionalOutputsOnly() OutputsByAddressAndOutputID {
	now := time.Now()
	result := NewAddressToOutputs()
	for addy, IDToOutputMap := range o {
		for outputID, output := range IDToOutputMap {
			switch output.Object.Type() {
			case ledgerstate.ExtendedLockedOutputType:
				casted := output.Object.(*ledgerstate.ExtendedLockedOutput)
				_, fallbackDeadline := casted.FallbackOptions()
				if !fallbackDeadline.IsZero() && addy.Address().Equals(casted.UnlockAddressNow(now)) {
					if _, addressExists := result[addy]; !addressExists {
						result[addy] = make(map[ledgerstate.OutputID]*Output)
					}
					result[addy][outputID] = output
				}
			}
		}
	}
	return result
}

func (o OutputsByAddressAndOutputID) AliasOutputsOnly() OutputsByAddressAndOutputID {
	result := NewAddressToOutputs()
	for addy, IDToOutputMap := range o {
		for outputID, output := range IDToOutputMap {
			switch output.Object.Type() {
			case ledgerstate.AliasOutputType:
				if _, addressExists := result[addy]; !addressExists {
					result[addy] = make(map[ledgerstate.OutputID]*Output)
				}
				result[addy][outputID] = output
			}
		}
	}
	return result
}

func (o OutputsByAddressAndOutputID) TotalFundsInOutputs() map[ledgerstate.Color]uint64 {
	result := make(map[ledgerstate.Color]uint64)
	for _, IDToOutputMap := range o {
		for _, output := range IDToOutputMap {
			output.Object.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				result[color] += balance
				return true
			})
		}
	}
	return result
}

func (o OutputsByAddressAndOutputID) ToLedgerStateOutputs() ledgerstate.Outputs {
	outputs := ledgerstate.Outputs{}
	for _, outputIDMapping := range o {
		for _, output := range outputIDMapping {
			outputs = append(outputs, output.Object)
		}
	}
	return outputs
}
