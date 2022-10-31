package mana

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type Tracker struct {
	ledger              *ledger.Ledger
	chainStorage        *storage.Storage
	accessManaVector    *manamodels.ManaBaseVector
	consensusManaVector *manamodels.ManaBaseVector

	cManaTargetEpoch epoch.Index
}

func NewTracker(l *ledger.Ledger, chainStorage *storage.Storage, opts ...options.Option[Tracker]) (manaTracker *Tracker) {
	return options.Apply(&Tracker{
		ledger:              l,
		chainStorage:        chainStorage,
		accessManaVector:    manamodels.NewManaBaseVector(manamodels.AccessMana),
		consensusManaVector: manamodels.NewManaBaseVector(manamodels.ConsensusMana),
	}, opts)
}

func (t *Tracker) UpdateConsensusWeights(weightUpdates map[identity.ID]*models.TimedBalance) {
	t.consensusManaVector.Lock()
	defer t.consensusManaVector.Unlock()

	for id, updateMana := range weightUpdates {
		t.consensusManaVector.SetMana(id, manamodels.NewManaBase(updateMana.Balance))
	}
}

func (t *Tracker) UpdateMana(txMeta *ledger.TransactionMetadata) {
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

func (t *Tracker) bookAccessMana(txInfo *manamodels.TxInfo) {
	t.accessManaVector.Lock()
	defer t.accessManaVector.Unlock()

	for _, inputInfo := range txInfo.InputInfos {
		t.accessManaVector.GetOldManaAndRevoke(inputInfo.PledgeID[manamodels.AccessMana], inputInfo.Amount)
	}
	t.accessManaVector.GetOldManaAndPledge(txInfo.PledgeID[manamodels.AccessMana], txInfo.TotalBalance)
}
