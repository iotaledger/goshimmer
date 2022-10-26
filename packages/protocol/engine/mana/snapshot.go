package mana

import (
	"errors"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/core/identity"
)

func (t *Tracker) LoadOutputsWithMetadata(outputsWithMetadata []*chainstorage.OutputWithMetadata) {
	t.processOutputs(outputsWithMetadata, manamodels.ConsensusMana, true)
	t.processOutputs(outputsWithMetadata, manamodels.AccessMana, true)
}

func (t *Tracker) RollbackOutputs(index epoch.Index, outputsWithMetadata []*chainstorage.OutputWithMetadata, areCreated bool) {
	t.processOutputs(outputsWithMetadata, manamodels.ConsensusMana, !areCreated)
	if index > t.chainStorage.LatestCommittedEpoch() {
		t.processOutputs(outputsWithMetadata, manamodels.AccessMana, !areCreated)
	}
}

func (t *Tracker) processOutputs(outputsWithMetadata []*chainstorage.OutputWithMetadata, manaType manamodels.Type, areCreated bool) {
	for _, outputWithMetadata := range outputsWithMetadata {
		devnetOutput := outputWithMetadata.Output().(devnetvm.Output)
		balance, exists := devnetOutput.Balances().Get(devnetvm.ColorIOTA)
		// TODO: shouldn't it get all balances of all colored coins instead of only IOTA?
		if !exists {
			continue
		}

		baseVector := t.baseManaVectors[manaType]

		var pledgeID identity.ID
		switch manaType {
		case manamodels.AccessMana:
			pledgeID = outputWithMetadata.AccessManaPledgeID()
		case manamodels.ConsensusMana:
			pledgeID = outputWithMetadata.ConsensusManaPledgeID()
		default:
			panic("invalid mana type")
		}

		existingMana, _, err := baseVector.GetMana(pledgeID)
		if !errors.Is(err, manamodels.ErrIssuerNotFoundInBaseManaVector) {
			continue
		}
		if areCreated {
			existingMana += int64(balance)
		} else {
			existingMana -= int64(balance)
		}
		baseVector.SetMana(pledgeID, manamodels.NewManaBase(existingMana))
	}
}