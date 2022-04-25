package manarefresher

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm/indexer"
)

// DelegationReceiver checks for delegation outputs on the wallet address and keeps the most recent delegated balance.
type DelegationReceiver struct {
	*wallet
	delegatedFunds map[devnetvm.Color]uint64
	delFundsMutex  sync.RWMutex
	sync.RWMutex

	// utility to be able to filter based on the same timestamp
	localTimeNow time.Time
}

// Scan scans for unspent delegation outputs on the delegation receiver address.
func (d *DelegationReceiver) Scan() []*devnetvm.AliasOutput {
	d.Lock()
	defer d.Unlock()

	var outputs devnetvm.Outputs
	deps.Indexer.CachedAddressOutputMappings(d.Address()).Consume(func(mapping *indexer.AddressOutputMapping) {
		deps.Tangle.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output *ledger.Output) {
			if typedOutput, ok := output.Output.(*devnetvm.Output); ok {
				outputs = append(outputs, typedOutput)
			}
		})
	})
	// filterDelegationOutputs will use this time for condition checking
	d.localTimeNow = clock.SyncedTime()
	filtered := outputs.Filter(d.filterDelegationOutputs)

	scanResult := make([]*devnetvm.AliasOutput, len(filtered))
	for i, output := range filtered {
		scanResult[i] = output.Clone().(*devnetvm.AliasOutput)
	}
	d.updateDelegatedFunds(scanResult)
	return scanResult
}

// updateDelegatedFunds updates the internal store of the delegated amount.
func (d *DelegationReceiver) updateDelegatedFunds(delegatedOutputs []*devnetvm.AliasOutput) {
	d.delFundsMutex.Lock()
	defer d.delFundsMutex.Unlock()
	current := map[devnetvm.Color]uint64{}
	for _, alias := range delegatedOutputs {
		alias.Balances().ForEach(func(color devnetvm.Color, balance uint64) bool {
			current[color] += balance
			return true
		})
	}
	d.delegatedFunds = current
}

// TotalDelegatedFunds returns the total amount of funds currently delegated to this node.
func (d *DelegationReceiver) TotalDelegatedFunds() uint64 {
	d.delFundsMutex.RLock()
	defer d.delFundsMutex.RUnlock()
	total := uint64(0)
	for _, balance := range d.delegatedFunds {
		total += balance
	}
	return total
}

// Address returns the receive address of the delegation receiver.
func (d *DelegationReceiver) Address() devnetvm.Address {
	return d.address
}

// filterDelegationOutputs checks if the output satisfies the conditions to be considered a valid delegation outputs
// that the plugin can refresh:
//		- output is an AliasOutput
// 		- it is unspent
// 		- it is confirmed
// 		- its state address is the same as DelegationReceiver's address
//		- output is delegated
// 		- if delegation time lock is present, it doesn't expire within 1 minute
func (d *DelegationReceiver) filterDelegationOutputs(output devnetvm.Output) bool {
	// it has to be an alias
	if output.Type() != devnetvm.AliasOutputType {
		return false
	}
	// it has to be unspent
	isUnspent := false
	isConfirmed := false
	deps.Tangle.Ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
		isUnspent = !outputMetadata.IsSpent()
		isConfirmed = deps.Tangle.ConfirmationOracle.IsOutputConfirmed(output.ID())
	})
	if !isUnspent || !isConfirmed {
		return false
	}
	// has to be a delegation alias that the delegation address owns for at least 1 min into the future
	alias := output.(*devnetvm.AliasOutput)
	if !alias.GetStateAddress().Equals(d.address) {
		return false
	}
	if !alias.IsDelegated() {
		return false
	}
	// when delegation timelock is present, we want to have 1 minute window to prepare the refresh tx, otherwise just drop it
	// TODO: what is the optimal window?
	if !alias.DelegationTimelock().IsZero() && !alias.DelegationTimeLockedNow(d.localTimeNow.Add(time.Minute)) {
		return false
	}
	return true
}
