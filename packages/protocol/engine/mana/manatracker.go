package mana

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type Tracker struct {
	ledger       *ledger.Ledger
	chainStorage *storage.Storage
	manaByID     *shrinkingmap.ShrinkingMap[identity.ID, int64]
	totalMana    int64

	cManaTargetEpoch epoch.Index

	sync.RWMutex
}

func NewTracker(l *ledger.Ledger, chainStorage *storage.Storage, opts ...options.Option[Tracker]) (manaTracker *Tracker) {
	return options.Apply(&Tracker{
		ledger:       l,
		chainStorage: chainStorage,
		manaByID:     shrinkingmap.New[identity.ID, int64](),
	}, opts)
}

func (t *Tracker) UpdateMana(txMeta *ledger.TransactionMetadata) {
	t.ledger.Storage.CachedTransaction(txMeta.ID()).Consume(func(transaction utxo.Transaction) {
		devnetTransaction := transaction.(*devnetvm.Transaction)

		// process transaction object to build txInfo
		pledgeFrom := t.gatherInputInfos(devnetTransaction.Essence().Inputs())

		// only book AccessMana
		t.bookAccessMana(devnetTransaction.Essence().AccessPledgeID(), pledgeFrom)
	})
}

func (t *Tracker) ManaMap() (manaMap map[identity.ID]int64) {
	t.RLock()
	defer t.RUnlock()

	return t.manaByID.AsMap()
}

func (t *Tracker) Mana(id identity.ID) (mana int64, exists bool) {
	t.RLock()
	defer t.RUnlock()

	return t.manaByID.Get(id)
}

func (t *Tracker) TotalMana() (totalMana int64) {
	return t.totalMana
}

func (t *Tracker) gatherInputInfos(inputs devnetvm.Inputs) (pledgeFrom map[identity.ID]int64) {
	pledgeFrom = make(map[identity.ID]int64)
	for _, input := range inputs {
		t.ledger.Storage.CachedOutput(input.(*devnetvm.UTXOInput).ReferencedOutputID()).Consume(func(o utxo.Output) {
			// look into the transaction, we need timestamp and access & consensus pledge IDs
			t.ledger.Storage.CachedOutputMetadata(o.ID()).Consume(func(metadata *ledger.OutputMetadata) {
				if amount, exists := o.(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
					pledgeFrom[metadata.AccessManaPledgeID()] += int64(amount)
				}
			})
		})
	}
	return
}

func (t *Tracker) bookAccessMana(pledgeID identity.ID, pledgeFrom map[identity.ID]int64) {
	t.Lock()
	defer t.Unlock()

	pledgedAmount := int64(0)
	for revokeID, revokedAmount := range pledgeFrom {
		if newBalance := lo.Return1(t.manaByID.Get(revokeID)) - revokedAmount; newBalance < 0 {
			t.manaByID.Delete(revokeID)
		} else {
			t.manaByID.Set(revokeID, newBalance)
		}

		pledgedAmount += revokedAmount
	}

	t.manaByID.Set(pledgeID, lo.Return1(t.manaByID.Get(pledgeID))+pledgedAmount)
}
