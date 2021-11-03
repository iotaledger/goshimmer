package wallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// OutputManager keeps track of the outputs.
type OutputManager struct {
	addressManager *AddressManager
	connector      Connector
	unspentOutputs OutputsByAddressAndOutputID
}

// NewUnspentOutputManager creates a new UnspentOutputManager.
func NewUnspentOutputManager(addressManager *AddressManager, connector Connector) (outputManager *OutputManager) {
	outputManager = &OutputManager{
		addressManager: addressManager,
		connector:      connector,
		unspentOutputs: NewAddressToOutputs(),
	}

	if err := outputManager.Refresh(true); err != nil {
		panic(err)
	}

	return
}

// Refresh checks for unspent outputs on the addresses provided by address manager and updates the internal state.
func (o *OutputManager) Refresh(includeSpentAddresses ...bool) error {
	// go through the list of all addresses in the wallet
	addressesToRefresh := o.addressManager.Addresses()

	// fetch unspent outputs on these addresses
	unspentOutputs, err := o.connector.UnspentOutputs(addressesToRefresh...)
	if err != nil {
		return err
	}

	// update addressmanager's internal store
	for addy, IDToOutputMap := range unspentOutputs {
		for outputID, output := range IDToOutputMap {
			if _, addressExists := o.unspentOutputs[addy]; !addressExists {
				o.unspentOutputs[addy] = make(map[ledgerstate.OutputID]*Output)
			}
			// mark the output as spent if we already marked it as spent locally
			if existingOutput, outputExists := o.unspentOutputs[addy][outputID]; outputExists && existingOutput.Spent {
				output.Spent = true
			}
			o.unspentOutputs[addy][outputID] = output
		}
	}

	return nil
}

// UnspentOutputs returns the all outputs that have not been spent, yet.
func (o *OutputManager) UnspentOutputs(includePending bool, addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID) {
	return o.getOutputs(includePending, addresses...)
}

// UnspentValueOutputs returns the value type outputs that have not been spent, yet.
func (o *OutputManager) UnspentValueOutputs(includePending bool, addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID) {
	return o.getOutputs(includePending, addresses...).ValueOutputsOnly()
}

// UnspentConditionalOutputs returns the ExtendedLockedoutputs that are conditionally owned by the wallet right now and
// have not been spent yet. Such outputs can be claimed by the wallet.
func (o *OutputManager) UnspentConditionalOutputs(includePending bool, addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID) {
	return o.getOutputs(includePending, addresses...).ConditionalOutputsOnly()
}

// UnspentAliasOutputs returns the alias type outputs that have not been spent, yet.
func (o *OutputManager) UnspentAliasOutputs(includePending bool, addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID) {
	return o.getOutputs(includePending, addresses...).AliasOutputsOnly()
}

func (o *OutputManager) getOutputs(includePending bool, addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID) {
	// prepare result
	unspentOutputs = make(map[address.Address]map[ledgerstate.OutputID]*Output)

	// retrieve the list of addresses from the address manager if none was provided
	if len(addresses) == 0 {
		addresses = o.addressManager.Addresses()
	}

	// iterate through addresses and scan for unspent outputs
	for _, addr := range addresses {
		// skip the address if we have no outputs for it stored
		unspentOutputsOnAddress, addressExistsInStoredOutputs := o.unspentOutputs[addr]
		if !addressExistsInStoredOutputs {
			continue
		}

		// iterate through outputs
		for transactionID, output := range unspentOutputsOnAddress {
			// skip spent outputs
			if output.Spent {
				continue
			}
			// discard non-confirmed if includePending is false
			if !includePending && !output.GradeOfFinalityReached {
				continue
			}

			// store unspent outputs in result
			if _, addressExists := unspentOutputs[addr]; !addressExists {
				unspentOutputs[addr] = make(map[ledgerstate.OutputID]*Output)
			}
			unspentOutputs[addr][transactionID] = output
		}
	}

	return unspentOutputs
}

// MarkOutputSpent marks the output identified by the given parameters as spent.
func (o *OutputManager) MarkOutputSpent(addy address.Address, outputID ledgerstate.OutputID) {
	// abort if we try to mark an unknown output as spent
	if _, addressExists := o.unspentOutputs[addy]; !addressExists {
		return
	}
	output, outputExists := o.unspentOutputs[addy][outputID]
	if !outputExists {
		return
	}

	// mark output as spent
	output.Spent = true
}
