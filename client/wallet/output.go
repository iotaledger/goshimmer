package wallet

import (
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// Output is a wallet specific representation of an output in the IOTA network.
type Output struct {
	Address                  address.Address
	Object                   devnetvm.Output
	Metadata                 OutputMetadata
	ConfirmationStateReached bool
	// Spent is a local wallet-only property that gets set once an output is spent from within the same wallet.
	Spent bool
}

// OutputMetadata is metadata about the output.
type OutputMetadata struct {
	// Timestamp is the timestamp of the tx that created the output.
	Timestamp time.Time
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsByID //////////////////////////////////////////////////////////////////////////////////////////////////

// OutputsByID is a collection of Outputs associated with their OutputID.
type OutputsByID map[utxo.OutputID]*Output

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

type OutputsByAddressAndOutputID map[address.Address]map[utxo.OutputID]*Output

// NewAddressToOutputs creates an empty container.
func NewAddressToOutputs() OutputsByAddressAndOutputID {
	return make(map[address.Address]map[utxo.OutputID]*Output)
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

// ValueOutputsOnly filters out non-value type outputs (aliases).
func (o OutputsByAddressAndOutputID) ValueOutputsOnly() OutputsByAddressAndOutputID {
	result := NewAddressToOutputs()
	for addy, IDToOutputMap := range o {
		for outputID, output := range IDToOutputMap {
			switch output.Object.Type() {
			case devnetvm.SigLockedSingleOutputType, devnetvm.SigLockedColoredOutputType, devnetvm.ExtendedLockedOutputType:
				if _, addressExists := result[addy]; !addressExists {
					result[addy] = make(map[utxo.OutputID]*Output)
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
			if output.Object.Type() == devnetvm.ExtendedLockedOutputType {
				casted := output.Object.(*devnetvm.ExtendedLockedOutput)
				_, fallbackDeadline := casted.FallbackOptions()
				if !fallbackDeadline.IsZero() && addy.Address().Equals(casted.UnlockAddressNow(now)) {
					if _, addressExists := result[addy]; !addressExists {
						result[addy] = make(map[utxo.OutputID]*Output)
					}
					result[addy][outputID] = output
				}
			}
		}
	}
	return result
}

// AliasOutputsOnly filters out any non-alias outputs.
func (o OutputsByAddressAndOutputID) AliasOutputsOnly() OutputsByAddressAndOutputID {
	result := NewAddressToOutputs()
	for addy, IDToOutputMap := range o {
		for outputID, output := range IDToOutputMap {
			if output.Object.Type() == devnetvm.AliasOutputType {
				if _, addressExists := result[addy]; !addressExists {
					result[addy] = make(map[utxo.OutputID]*Output)
				}
				result[addy][outputID] = output
			}
		}
	}
	return result
}

// TotalFundsInOutputs returns the total funds present in the outputs.
func (o OutputsByAddressAndOutputID) TotalFundsInOutputs() map[devnetvm.Color]uint64 {
	result := make(map[devnetvm.Color]uint64)
	for _, IDToOutputMap := range o {
		for _, output := range IDToOutputMap {
			output.Object.Balances().ForEach(func(color devnetvm.Color, balance uint64) bool {
				result[color] += balance
				return true
			})
		}
	}
	return result
}

// ToLedgerStateOutputs transforms all outputs in the mapping into a slice of ledgerstate outputs.
func (o OutputsByAddressAndOutputID) ToLedgerStateOutputs() devnetvm.Outputs {
	outputs := devnetvm.Outputs{}
	for _, outputIDMapping := range o {
		for _, output := range outputIDMapping {
			outputs = append(outputs, output.Object)
		}
	}
	return outputs
}

// OutputCount returns the number of outputs in the struct.
func (o OutputsByAddressAndOutputID) OutputCount() int {
	outputCount := 0
	for _, outputIDMapping := range o {
		for range outputIDMapping {
			outputCount++
		}
	}
	return outputCount
}

// SplitIntoChunksOfMaxInputCount splits the mapping into chunks that contain at most ledgerstate.MaxInputCount outputs.
func (o OutputsByAddressAndOutputID) SplitIntoChunksOfMaxInputCount() []OutputsByAddressAndOutputID {
	outputCount := o.OutputCount()
	if outputCount <= devnetvm.MaxInputCount {
		// there is no need to split
		return []OutputsByAddressAndOutputID{o}
	}
	result := make([]OutputsByAddressAndOutputID, outputCount/devnetvm.MaxInputCount+1)
	for i := range result {
		// init all chunks
		result[i] = NewAddressToOutputs()
	}
	processedCount := 0
	chunkCount := -1
	for addy, outputIDMapping := range o {
		for outputID, output := range outputIDMapping {
			if processedCount%devnetvm.MaxInputCount == 0 {
				chunkCount++
			}
			if _, has := result[chunkCount][addy]; !has {
				result[chunkCount][addy] = make(map[utxo.OutputID]*Output)
			}
			result[chunkCount][addy][outputID] = output
			processedCount++
		}
	}
	return result
}
