package mana

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

type Tracker struct {
	ledger          *ledger.Ledger
	chainStorage    *chainstorage.ChainStorage
	baseManaVectors map[manamodels.Type]*manamodels.ManaBaseVector

	Events           *Events
	cManaTargetEpoch epoch.Index
}

func NewTracker(l *ledger.Ledger, chainStorage *chainstorage.ChainStorage, opts ...options.Option[Tracker]) (manaTracker *Tracker) {
	return options.Apply(&Tracker{
		Events:          NewEvents(),
		ledger:          l,
		chainStorage:    chainStorage,
		baseManaVectors: make(map[manamodels.Type]*manamodels.ManaBaseVector),
	}, opts, func(m *Tracker) {
		m.baseManaVectors[manamodels.AccessMana] = manamodels.NewManaBaseVector(manamodels.AccessMana)
		m.baseManaVectors[manamodels.ConsensusMana] = manamodels.NewManaBaseVector(manamodels.ConsensusMana)
	})
}

func (t *Tracker) OnConsensusWeightsUpdated(event *chainstorage.ConsensusWeightsUpdatedEvent) {
	creationTime := event.EI.EndTime()

	t.applyUpdatesToConsensusVector(event.AmountAndDiffByIdentity)

	for id, manaUpdate := range event.AmountAndDiffByIdentity {
		switch {
		case manaUpdate.Diff == 0:
			continue
		case manaUpdate.Diff > 0:
			t.Events.Pledged.Trigger(&PledgedEvent{
				IssuerID: id,
				Amount:   manaUpdate.Balance,
				Time:     creationTime,
				ManaType: manamodels.ConsensusMana,
			})
		case manaUpdate.Diff < 0:
			t.Events.Revoked.Trigger(&RevokedEvent{
				IssuerID: id,
				Amount:   manaUpdate.OldAmount + manaUpdate.Diff,
				Time:     creationTime,
				ManaType: manamodels.ConsensusMana,
			})
		}

		t.Events.Updated.Trigger(&UpdatedEvent{
			IssuerID: id,
			OldMana:  manamodels.NewManaBase(manaUpdate.OldAmount),
			NewMana:  manamodels.NewManaBase(manaUpdate.OldAmount + manaUpdate.Diff),
			ManaType: manamodels.ConsensusMana,
		})
	}
}

func (t *Tracker) OnTransactionAccepted(txMeta *ledger.TransactionMetadata) {
	t.ledger.Storage.CachedTransaction(txMeta.ID()).Consume(func(transaction utxo.Transaction) {
		devnetTransaction := transaction.(*devnetvm.Transaction)

		// process transaction object to build txInfo
		totalAmount, inputInfos := t.gatherInputInfos(devnetTransaction.Essence().Inputs())

		// only book AccessMana
		t.bookAccessMana(&manamodels.TxInfo{
			TimeStamp:     devnetTransaction.Essence().Timestamp(),
			TransactionID: txMeta.ID(),
			TotalBalance:  totalAmount,
			PledgeID: map[manamodels.Type]identity.ID{
				manamodels.AccessMana:    devnetTransaction.Essence().AccessPledgeID(),
				manamodels.ConsensusMana: devnetTransaction.Essence().ConsensusPledgeID(),
			},
			InputInfos: inputInfos,
		})
	})
}

// bookAccessMana books access mana for a transaction.
func (t *Tracker) bookAccessMana(txInfo *manamodels.TxInfo) {
	revokeEvents, pledgeEvents, updateEvents := t.bookTransaction(txInfo)

	t.triggerManaEvents(revokeEvents, pledgeEvents, updateEvents)
}

func (t *Tracker) gatherInputInfos(inputs devnetvm.Inputs) (totalAmount int64, inputInfos []manamodels.InputInfo) {
	inputInfos = make([]manamodels.InputInfo, 0)
	for _, input := range inputs {
		var inputInfo manamodels.InputInfo

		outputID := input.(*devnetvm.UTXOInput).ReferencedOutputID()
		t.ledger.Storage.CachedOutput(outputID).Consume(func(o utxo.Output) {
			inputInfo.InputID = o.ID()

			// first, sum balances of the input, calculate total amount as well for later
			if amount, exists := o.(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
				inputInfo.Amount = int64(amount)
				totalAmount += int64(amount)
			}

			// look into the transaction, we need timestamp and access & consensus pledge IDs
			t.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(metadata *ledger.OutputMetadata) {
				inputInfo.PledgeID = map[manamodels.Type]identity.ID{
					manamodels.AccessMana:    metadata.AccessManaPledgeID(),
					manamodels.ConsensusMana: metadata.ConsensusManaPledgeID(),
				}
			})
		})
		inputInfos = append(inputInfos, inputInfo)
	}
	return totalAmount, inputInfos
}

func (t *Tracker) bookTransaction(txInfo *manamodels.TxInfo) (revokeEvents []*RevokedEvent, pledgeEvents []*PledgedEvent, updateEvents []*UpdatedEvent) {
	accessManaVector := t.baseManaVectors[manamodels.AccessMana]

	accessManaVector.Lock()
	defer accessManaVector.Unlock()
	// first, revoke mana from previous owners
	for _, inputInfo := range txInfo.InputInfos {
		// which issuer did the input pledge mana to?
		oldPledgeIssuerID := inputInfo.PledgeID[manamodels.AccessMana]
		oldMana := accessManaVector.GetOldManaAndRevoke(oldPledgeIssuerID, inputInfo.Amount)
		// save events for later triggering
		revokeEvents = append(revokeEvents, &RevokedEvent{
			IssuerID:      oldPledgeIssuerID,
			Amount:        inputInfo.Amount,
			Time:          txInfo.TimeStamp,
			ManaType:      manamodels.AccessMana,
			TransactionID: txInfo.TransactionID,
			InputID:       inputInfo.InputID,
		})
		updateEvents = append(updateEvents, &UpdatedEvent{
			IssuerID: oldPledgeIssuerID,
			OldMana:  &oldMana,
			NewMana:  accessManaVector.M.Vector[oldPledgeIssuerID],
			ManaType: manamodels.AccessMana,
		})
	}
	// second, pledge mana to new issuers
	newPledgeIssuerID := txInfo.PledgeID[manamodels.AccessMana]
	oldMana := accessManaVector.GetOldManaAndPledge(newPledgeIssuerID, txInfo.TotalBalance)

	pledgeEvents = append(pledgeEvents, &PledgedEvent{
		IssuerID: newPledgeIssuerID,
		Amount:   txInfo.SumInputs(),
		Time:     txInfo.TimeStamp,
		ManaType: manamodels.AccessMana,
	})
	updateEvents = append(updateEvents, &UpdatedEvent{
		IssuerID: newPledgeIssuerID,
		OldMana:  &oldMana,
		NewMana:  accessManaVector.M.Vector[newPledgeIssuerID],
		ManaType: manamodels.AccessMana,
	})

	return revokeEvents, pledgeEvents, updateEvents
}

// BookEpoch takes care of the booking of consensus mana for the given committed epoch.
func (t *Tracker) applyUpdatesToConsensusVector(weightsUpdate map[identity.ID]*notarization.ConsensusWeightUpdate) {
	consensusManaVector := t.baseManaVectors[manamodels.ConsensusMana]
	consensusManaVector.Lock()
	defer consensusManaVector.Unlock()

	for id, updateMana := range weightsUpdate {
		if updateMana.Diff < 0 {
			consensusManaVector.GetOldManaAndRevoke(id, -updateMana.Diff)
		} else {
			consensusManaVector.GetOldManaAndPledge(id, updateMana.Diff)
		}
	}
}

func (t *Tracker) triggerManaEvents(revokeEvents []*RevokedEvent, pledgeEvents []*PledgedEvent, updateEvents []*UpdatedEvent) {
	// trigger the events once we released the lock on the mana vector
	for _, ev := range revokeEvents {
		t.Events.Revoked.Trigger(ev)
	}
	for _, ev := range pledgeEvents {
		t.Events.Pledged.Trigger(ev)
	}
	for _, ev := range updateEvents {
		t.Events.Updated.Trigger(ev)
	}
}

func (t *Tracker) cleanupManaVectors() {
	for _, vecType := range []manamodels.Type{manamodels.AccessMana, manamodels.ConsensusMana} {
		manaBaseVector := t.baseManaVectors[vecType]
		manaBaseVector.RemoveZeroIssuers()
	}
}
