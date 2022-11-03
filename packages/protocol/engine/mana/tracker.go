package mana

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type Tracker struct {
	totalMana int64
	manaByID  *shrinkingmap.ShrinkingMap[identity.ID, int64]
	ledger    *ledger.Ledger

	sync.RWMutex
}

func NewTracker(ledger *ledger.Ledger, opts ...options.Option[Tracker]) (manaTracker *Tracker) {
	return options.Apply(&Tracker{
		manaByID: shrinkingmap.New[identity.ID, int64](),
		ledger:   ledger,
	}, opts)
}

func (t *Tracker) ProcessAcceptedTransaction(metadata *ledger.TransactionMetadata) {
	t.ledger.Storage.CachedTransaction(metadata.ID()).Consume(func(transaction utxo.Transaction) {
		txEssence := transaction.(*devnetvm.Transaction).Essence()

		t.updateMana(txEssence.AccessPledgeID(), t.revokeManaFromInputs(txEssence.Inputs()))
	})
}

func (t *Tracker) Mana(id identity.ID) (mana int64, exists bool) {
	t.RLock()
	defer t.RUnlock()

	return t.manaByID.Get(id)
}

func (t *Tracker) ManaByID() (manaByID map[identity.ID]int64) {
	t.RLock()
	defer t.RUnlock()

	return t.manaByID.AsMap()
}

func (t *Tracker) TotalMana() (totalMana int64) {
	return t.totalMana
}

func (t *Tracker) ImportOutputsFromSnapshot(outputs []*models.OutputWithMetadata) {
	t.totalMana += t.processOutputsFromSnapshot(outputs, true)
}

func (t *Tracker) RollbackOutputsFromSnapshot(outputs []*models.OutputWithMetadata, areCreated bool) {
	t.processOutputsFromSnapshot(outputs, !areCreated)
}

func (t *Tracker) updateMana(id identity.ID, diff int64) {
	t.Lock()
	defer t.Unlock()

	if newBalance := lo.Return1(t.manaByID.Get(id)) + diff; newBalance != 0 {
		t.manaByID.Set(id, newBalance)
	} else {
		t.manaByID.Delete(id)
	}
}

func (t *Tracker) revokeManaFromInputs(inputs devnetvm.Inputs) (totalRevoked int64) {
	for _, input := range inputs {
		t.ledger.Storage.CachedOutput(input.(*devnetvm.UTXOInput).ReferencedOutputID()).Consume(func(o utxo.Output) {
			t.ledger.Storage.CachedOutputMetadata(o.ID()).Consume(func(metadata *ledger.OutputMetadata) {
				if amount, exists := o.(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
					t.updateMana(metadata.AccessManaPledgeID(), -int64(amount))

					totalRevoked += int64(amount)
				}
			})
		})
	}

	return
}

func (t *Tracker) processOutputsFromSnapshot(outputs []*models.OutputWithMetadata, areCreated bool) (totalDiff int64) {
	for _, output := range outputs {
		if iotaBalance, exists := output.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
			diff := lo.Cond(areCreated, int64(iotaBalance), -int64(iotaBalance))
			totalDiff += diff

			t.updateMana(output.AccessManaPledgeID(), diff)
		}
	}

	return
}
