package mana1

/*

import (
	"fmt"
	"testing"
	"time"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

var (
	baseTime            = time.Now()
	txTime              = baseTime.Add(time.Hour * 6)
	txPledgeID          = randIssuerID()
	inputPledgeID1      = randIssuerID()
	inputPledgeID2      = randIssuerID()
	inputPledgeID3      = randIssuerID()
	beforeBookingAmount = map[identity.ID]int64{
		txPledgeID:     0,
		inputPledgeID1: 5.0,
		inputPledgeID2: 3.0,
		inputPledgeID3: 2.0,
	}
	afterBookingAmount = map[identity.ID]int64{
		txPledgeID:     10.0,
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}
	txInfo = &manamodels.TxInfo{
		TimeStamp:     txTime,
		TransactionID: randomTxID(),
		TotalBalance:  10.0,
		PledgeID: map[manamodels.Type]identity.ID{
			manamodels.AccessMana:    txPledgeID,
			manamodels.ConsensusMana: txPledgeID,
		},
		InputInfos: []manamodels.InputInfo{
			{
				Amount: beforeBookingAmount[inputPledgeID1],
				PledgeID: map[manamodels.Type]identity.ID{
					manamodels.AccessMana:    inputPledgeID1,
					manamodels.ConsensusMana: inputPledgeID1,
				},
				InputID: utxo.NewOutputID(randomTxID(), 0),
			},
			{
				Amount: beforeBookingAmount[inputPledgeID2],
				PledgeID: map[manamodels.Type]identity.ID{
					manamodels.AccessMana:    inputPledgeID2,
					manamodels.ConsensusMana: inputPledgeID2,
				},
				InputID: utxo.NewOutputID(randomTxID(), 0),
			},
			{
				Amount: beforeBookingAmount[inputPledgeID3],
				PledgeID: map[manamodels.Type]identity.ID{
					manamodels.AccessMana:    inputPledgeID3,
					manamodels.ConsensusMana: inputPledgeID3,
				},
				InputID: utxo.NewOutputID(randomTxID(), 0),
			},
		},
	}
	slotCreatedBalances    = []uint64{1, 1, 2, 2, 1}
	slotSpentBalances      = []uint64{2, 2, 2, 1}
	slotCreatedPledgeIDs   = []identity.ID{inputPledgeID1, inputPledgeID2, inputPledgeID2, inputPledgeID3, inputPledgeID3}
	slotSpentPledgeIDs     = []identity.ID{inputPledgeID1, inputPledgeID1, inputPledgeID2, inputPledgeID3}
	afterBookingSlotAmount = map[identity.ID]int64{
		inputPledgeID1: 2.0,
		inputPledgeID2: 4.0,
		inputPledgeID3: 4.0,
	}
)

func prepareSlotDiffs() (created []*chainstorage.OutputWithMetadata, spent []*chainstorage.OutputWithMetadata) {
	for i, amount := range slotCreatedBalances {
		outWithMeta := createOutputWithMetadata(amount, slotCreatedPledgeIDs[i])
		created = append(created, outWithMeta)
	}
	for i, amount := range slotSpentBalances {
		outWithMeta := createOutputWithMetadata(amount, slotSpentPledgeIDs[i])
		spent = append(spent, outWithMeta)
	}

	return
}

func createOutputWithMetadata(amount uint64, createdPledgeID identity.ID) *chainstorage.OutputWithMetadata {
	now := time.Now()
	addr := seed.NewSeed().Address(0).Address()
	out := devnetvm.NewSigLockedSingleOutput(amount, addr)
	outWithMeta := chainstorage.NewOutputWithMetadata(out.ID(), out, now, createdPledgeID, createdPledgeID)
	return outWithMeta
}

func randomTxID() (txID utxo.TransactionID) {
	_ = txID.FromRandomness()
	return
}

func randIssuerID() identity.ID {
	return identity.GenerateIdentity().ID()
}

func TestTracker_BookTransaction(t *testing.T) {
	tf := ledger.NewTestFramework(t)
	tracker := New(tf.Ledger)
	// hold information about which events triggered
	var (
		updateEvents []*UpdatedEvent
		revokeEvents []*RevokedEvent
		pledgeEvents []*PledgedEvent
	)

	// when an event triggers, add it to the log
	tracker.Events.Updated.Hook(event.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))
	tracker.Events.Revoked.Hook(event.NewClosure(func(ev *RevokedEvent) {
		revokeEvents = append(revokeEvents, ev)
	}))
	tracker.Events.Pledged.Hook(event.NewClosure(func(ev *PledgedEvent) {
		pledgeEvents = append(pledgeEvents, ev)
	}))

	bmv := manamodels.NewManaBaseVector(manamodels.AccessMana)

	// init vector to inputTime with pledged beforeBookingAmount
	bmv.SetMana(inputPledgeID1, manamodels.NewManaBase(beforeBookingAmount[inputPledgeID1]))
	bmv.SetMana(inputPledgeID2, manamodels.NewManaBase(beforeBookingAmount[inputPledgeID2]))
	bmv.SetMana(inputPledgeID3, manamodels.NewManaBase(beforeBookingAmount[inputPledgeID3]))
	tracker.baseManaVectors[manamodels.AccessMana] = bmv

	// drop all recorded updatedEvents
	updateEvents = []*UpdatedEvent{}

	// book mana with txInfo at txTime
	tracker.BookTransaction(txInfo)

	// expected issuerIDs to be called with each event
	updatedIssuerIds := map[identity.ID]interface{}{
		txPledgeID:     0,
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}
	pledgedIssuerIds := map[identity.ID]interface{}{
		txPledgeID: 0,
	}
	revokedIssuerIds := map[identity.ID]interface{}{
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}

	// update triggered for the 3 issuers that mana was revoked from, and once for the pledged
	assert.Equal(t, 4, len(updateEvents))
	assert.Equal(t, 1, len(pledgeEvents))
	assert.Equal(t, 3, len(revokeEvents))

	for _, ev := range updateEvents {
		// has the right type
		assert.Equal(t, manamodels.AccessMana, ev.ManaType)
		// base mana values are expected
		assert.Equal(t, afterBookingAmount[ev.IssuerID], ev.NewMana.BaseValue(), "incorrect mana amount for %s", ev.IssuerID)
		assert.Contains(t, updatedIssuerIds, ev.IssuerID)
		delete(updatedIssuerIds, ev.IssuerID)
	}
	assert.Empty(t, updatedIssuerIds)
	for _, ev := range pledgeEvents {
		assert.Equal(t, afterBookingAmount[ev.IssuerID], ev.Amount)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, manamodels.AccessMana, ev.ManaType)
		assert.Contains(t, pledgedIssuerIds, ev.IssuerID)
		delete(pledgedIssuerIds, ev.IssuerID)
	}
	assert.Empty(t, pledgedIssuerIds)
	for _, ev := range revokeEvents {
		assert.Equal(t, beforeBookingAmount[ev.IssuerID], ev.Amount)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, manamodels.AccessMana, ev.ManaType)
		assert.Contains(t, revokedIssuerIds, ev.IssuerID)
		delete(revokedIssuerIds, ev.IssuerID)
	}
	assert.Empty(t, revokedIssuerIds)
}

func TestTracker_BookSlot(t *testing.T) {
	tf := ledger.NewTestFramework(t)
	tracker := New(tf.Ledger)
	// hold information about which events triggered
	var (
		updateEvents []*UpdatedEvent
		revokeEvents []*RevokedEvent
		pledgeEvents []*PledgedEvent
	)
	issuerIds := map[identity.ID]types.Empty{
		inputPledgeID1: types.Void,
		inputPledgeID2: types.Void,
		inputPledgeID3: types.Void,
	}

	fmt.Println("all ids: ", inputPledgeID1.String(), ' ', inputPledgeID2.String(), ' ', inputPledgeID3.String())
	created, spent := prepareSlotDiffs()
	// when an event triggers, add it to the log
	tracker.Events.Updated.Hook(event.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))
	tracker.Events.Revoked.Hook(event.NewClosure(func(ev *RevokedEvent) {
		revokeEvents = append(revokeEvents, ev)
	}))
	tracker.Events.Pledged.Hook(event.NewClosure(func(ev *PledgedEvent) {
		pledgeEvents = append(pledgeEvents, ev)
	}))
	bmv := manamodels.NewManaBaseVector(manamodels.ConsensusMana)

	// init vector to inputTime with pledged beforeBookingAmount
	bmv.SetMana(inputPledgeID1, manamodels.NewManaBase(beforeBookingAmount[inputPledgeID1]))
	bmv.SetMana(inputPledgeID2, manamodels.NewManaBase(beforeBookingAmount[inputPledgeID2]))
	bmv.SetMana(inputPledgeID3, manamodels.NewManaBase(beforeBookingAmount[inputPledgeID3]))
	tracker.baseManaVectors[manamodels.ConsensusMana] = bmv

	tracker.BookSlot(created, spent)

	// update triggered for the 3 issuers that mana was revoked from, and once for the pledged
	assert.Equal(t, 9, len(updateEvents))
	assert.Equal(t, 5, len(pledgeEvents))
	assert.Equal(t, 4, len(revokeEvents))

	latestUpdateEvent := make(map[identity.ID]*UpdatedEvent)
	for _, ev := range updateEvents {
		latestUpdateEvent[ev.IssuerID] = ev
	}
	// check only the latest update event for each issuerID
	for _, ev := range latestUpdateEvent {
		// has the right type
		assert.Equal(t, manamodels.ConsensusMana, ev.ManaType)
		// base mana values are expected
		assert.Equal(t, afterBookingSlotAmount[ev.IssuerID], ev.NewMana.BaseValue(), "incorrect mana amount for %s", ev.IssuerID)
		assert.Contains(t, issuerIds, ev.IssuerID)
	}

	afterEventsAmount := make(map[identity.ID]int64)
	afterEventsAmount[inputPledgeID1] = beforeBookingAmount[inputPledgeID1]
	afterEventsAmount[inputPledgeID2] = beforeBookingAmount[inputPledgeID2]
	afterEventsAmount[inputPledgeID3] = beforeBookingAmount[inputPledgeID3]

	for i, ev := range revokeEvents {
		afterEventsAmount[slotSpentPledgeIDs[i]] -= ev.Amount
		assert.Equal(t, manamodels.ConsensusMana, ev.ManaType)
		assert.Contains(t, issuerIds, ev.IssuerID)
	}
	for i, ev := range pledgeEvents {
		afterEventsAmount[slotCreatedPledgeIDs[i]] += ev.Amount
		assert.Equal(t, manamodels.ConsensusMana, ev.ManaType)
		assert.Contains(t, issuerIds, ev.IssuerID)
	}
	// make sure pledge and revoke events balance changes are as expected
	for id := range afterBookingSlotAmount {
		assert.Equal(t, afterEventsAmount[id], afterBookingSlotAmount[id])
		m, _, err := bmv.GetMana(id)
		require.NoError(t, err)
		assert.Equal(t, afterBookingSlotAmount[id], m)
	}
}

*/
