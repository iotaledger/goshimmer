package mana

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

func (t *Tracker) LoadOutputsWithMetadata(outputsWithMetadata []*models.OutputWithMetadata) {
	// t.processOutputs(outputsWithMetadata, manamodels.ConsensusMana, true)
	totalDiff := t.processOutputs(outputsWithMetadata, true)
	t.totalMana += totalDiff
}

func (t *Tracker) RollbackOutputs(index epoch.Index, outputsWithMetadata []*models.OutputWithMetadata, areCreated bool) {
	// t.processOutputs(outputsWithMetadata, manamodels.ConsensusMana, !areCreated)
	t.processOutputs(outputsWithMetadata, !areCreated)
}

func (t *Tracker) processOutputs(outputsWithMetadata []*models.OutputWithMetadata, areCreated bool) (totalDiff int64) {
	for _, outputWithMetadata := range outputsWithMetadata {
		devnetOutput := outputWithMetadata.Output().(devnetvm.Output)
		diff, exists := devnetOutput.Balances().Get(devnetvm.ColorIOTA)
		// TODO: shouldn't it get all balances of all colored coins instead of only IOTA?
		if !exists {
			continue
		}

		pledgeID := outputWithMetadata.AccessManaPledgeID()

		balance, _ := t.manaByID.Get(pledgeID)
		if areCreated {
			balance += int64(diff)
			totalDiff += int64(diff)
		} else {
			balance -= int64(diff)
		}
		t.manaByID.Set(pledgeID, balance)
	}

	return
}
