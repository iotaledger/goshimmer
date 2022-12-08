package manatracker

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

// ManaTracker is the manager that tracks the mana balances of identities.
type ManaTracker struct {
	manaByID      *shrinkingmap.ShrinkingMap[identity.ID, int64]
	manaByIDMutex sync.RWMutex
	totalMana     int64
	ledger        *ledger.Ledger
}

// New creates a new ManaTracker.
func New(ledgerInstance *ledger.Ledger, opts ...options.Option[ManaTracker]) (manaTracker *ManaTracker) {
	return options.Apply(&ManaTracker{
		manaByID: shrinkingmap.New[identity.ID, int64](),
		ledger:   ledgerInstance,
	}, opts, func(m *ManaTracker) {
		ledgerInstance.Events.TransactionAccepted.Attach(event.NewClosure(m.ProcessAcceptedTransaction))
	})
}

// ProcessAcceptedTransaction processes the accepted transaction and updates the mana according to the pledges.
func (t *ManaTracker) ProcessAcceptedTransaction(metadata *ledger.TransactionMetadata) {
	t.ledger.Storage.CachedTransaction(metadata.ID()).Consume(func(transaction utxo.Transaction) {
		txEssence := transaction.(*devnetvm.Transaction).Essence()

		t.updateMana(txEssence.AccessPledgeID(), t.revokeManaFromInputs(txEssence.Inputs()))
	})
}

// Mana returns the mana balance of an identity.
func (t *ManaTracker) Mana(id identity.ID) (mana int64, exists bool) {
	t.manaByIDMutex.RLock()
	defer t.manaByIDMutex.RUnlock()

	return t.manaByID.Get(id)
}

// ManaByIDs returns the mana balances of all identities.
func (t *ManaTracker) ManaByIDs() (manaByID map[identity.ID]int64) {
	t.manaByIDMutex.RLock()
	defer t.manaByIDMutex.RUnlock()

	return t.manaByID.AsMap()
}

// TotalMana returns the total amount of mana.
func (t *ManaTracker) TotalMana() (totalMana int64) {
	return t.totalMana
}

// ImportOutputs imports the outputs from a snapshot and updates the mana balances and the total mana.
func (t *ManaTracker) ImportOutputs(outputs []*models.OutputWithMetadata) {
	t.totalMana += t.processOutputsFromSnapshot(outputs, true)
}

// RollbackOutputs rolls back the outputs from a snapshot and updates the mana balances.
func (t *ManaTracker) RollbackOutputs(outputs []*models.OutputWithMetadata, areCreated bool) {
	t.processOutputsFromSnapshot(outputs, !areCreated)
}

// updateMana adds the diff to the current mana balance of an identity .
func (t *ManaTracker) updateMana(id identity.ID, diff int64) {
	t.manaByIDMutex.Lock()
	defer t.manaByIDMutex.Unlock()

	if newBalance := lo.Return1(t.manaByID.Get(id)) + diff; newBalance != 0 {
		t.manaByID.Set(id, newBalance)
	} else {
		t.manaByID.Delete(id)
	}
}

// revokeManaFromInputs revokes the mana from the inputs of a transaction and returns the total amount that was revoked.
func (t *ManaTracker) revokeManaFromInputs(inputs devnetvm.Inputs) (totalRevoked int64) {
	for _, input := range inputs {
		t.ledger.Storage.CachedOutput(input.(*devnetvm.UTXOInput).ReferencedOutputID()).Consume(func(output utxo.Output) {
			t.ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(metadata *ledger.OutputMetadata) {
				if amount, exists := output.(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
					t.updateMana(metadata.AccessManaPledgeID(), -int64(amount))

					totalRevoked += int64(amount)
				}
			})
		})
	}

	return
}

// processOutputsFromSnapshot processes the outputs from a snapshot and updates the mana balances.
func (t *ManaTracker) processOutputsFromSnapshot(outputs []*models.OutputWithMetadata, areCreated bool) (totalDiff int64) {
	for _, output := range outputs {
		if iotaBalance, exists := output.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
			diff := lo.Cond(areCreated, int64(iotaBalance), -int64(iotaBalance))
			totalDiff += diff

			t.updateMana(output.AccessManaPledgeID(), diff)
		}
	}

	return
}
