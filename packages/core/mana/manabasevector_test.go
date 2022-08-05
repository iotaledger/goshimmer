package mana

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/types"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

var (
	baseTime            = time.Now()
	txTime              = baseTime.Add(time.Hour * 6)
	txPledgeID          = randNodeID()
	inputPledgeID1      = randNodeID()
	inputPledgeID2      = randNodeID()
	inputPledgeID3      = randNodeID()
	beforeBookingAmount = map[identity.ID]float64{
		txPledgeID:     0,
		inputPledgeID1: 5.0,
		inputPledgeID2: 3.0,
		inputPledgeID3: 2.0,
	}
	afterBookingAmount = map[identity.ID]float64{
		txPledgeID:     10.0,
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}
	txInfo = &TxInfo{
		TimeStamp:     txTime,
		TransactionID: randomTxID(),
		TotalBalance:  10.0,
		PledgeID: map[Type]identity.ID{
			AccessMana:    txPledgeID,
			ConsensusMana: txPledgeID,
		},
		InputInfos: []InputInfo{
			{
				Amount: beforeBookingAmount[inputPledgeID1],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID1,
					ConsensusMana: inputPledgeID1,
				},
				InputID: utxo.NewOutputID(randomTxID(), 0),
			},
			{
				Amount: beforeBookingAmount[inputPledgeID2],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID2,
					ConsensusMana: inputPledgeID2,
				},
				InputID: utxo.NewOutputID(randomTxID(), 0),
			},
			{
				Amount: beforeBookingAmount[inputPledgeID3],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID3,
					ConsensusMana: inputPledgeID3,
				},
				InputID: utxo.NewOutputID(randomTxID(), 0),
			},
		},
	}
	epochCreatedBalances    = []uint64{1, 1, 2, 2, 1}
	epochSpentBalances      = []uint64{2, 2, 2, 1}
	epochCreatedPledgeIDs   = []identity.ID{inputPledgeID1, inputPledgeID2, inputPledgeID2, inputPledgeID3, inputPledgeID3}
	epochSpentPledgeIDs     = []identity.ID{inputPledgeID1, inputPledgeID1, inputPledgeID2, inputPledgeID3}
	afterBookingEpochAmount = map[identity.ID]float64{
		inputPledgeID1: 2.0,
		inputPledgeID2: 4.0,
		inputPledgeID3: 4.0,
	}
)

func randNodeID() identity.ID {
	return identity.GenerateIdentity().ID()
}

func prepareEpochDiffs() (created []*ledger.OutputWithMetadata, spent []*ledger.OutputWithMetadata) {
	for i, amount := range epochCreatedBalances {
		outWithMeta := createOutputWithMetadata(amount, epochCreatedPledgeIDs[i])
		created = append(created, outWithMeta)
	}
	for i, amount := range epochSpentBalances {
		outWithMeta := createOutputWithMetadata(amount, epochSpentPledgeIDs[i])
		spent = append(spent, outWithMeta)
	}

	return
}

func createOutputWithMetadata(amount uint64, createdPledgeID identity.ID) *ledger.OutputWithMetadata {
	now := time.Now()
	addr := seed.NewSeed().Address(0).Address()
	out := devnetvm.NewSigLockedSingleOutput(amount, addr)
	outWithMeta := ledger.NewOutputWithMetadata(out.ID(), out, now, createdPledgeID, createdPledgeID)
	return outWithMeta
}

func TestNewBaseManaVector_Consensus(t *testing.T) {
	bmvCons := NewBaseManaVector(ConsensusMana)
	assert.Equal(t, ConsensusMana, bmvCons.Type())
	assert.Equal(t, map[identity.ID]*ManaBase{}, bmvCons.(*ManaBaseVector).M.Vector)
}

func TestConsensusBaseManaVector_Type(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)
	vectorType := bmv.Type()
	assert.Equal(t, ConsensusMana, vectorType)
}

func TestConsensusBaseManaVector_Size(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)
	assert.Equal(t, 0, bmv.Size())

	for i := 0; i < 10; i++ {
		bmv.SetMana(randNodeID(), NewManaBase(float64(i)))
	}
	assert.Equal(t, 10, bmv.Size())
}

func TestConsensusBaseManaVector_Has(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)
	randID := randNodeID()

	has := bmv.Has(randID)
	assert.False(t, has)

	bmv.SetMana(randID, NewManaBase(0))
	has = bmv.Has(randID)
	assert.True(t, has)
}

func TestConsensusBaseManaVector_Book(t *testing.T) {
	// hold information about which events triggered
	var (
		updateEvents []*UpdatedEvent
		revokeEvents []*RevokedEvent
		pledgeEvents []*PledgedEvent
	)

	// when an event triggers, add it to the log
	Events.Updated.Hook(event.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))
	Events.Revoked.Hook(event.NewClosure(func(ev *RevokedEvent) {
		revokeEvents = append(revokeEvents, ev)
	}))
	Events.Pledged.Hook(event.NewClosure(func(ev *PledgedEvent) {
		pledgeEvents = append(pledgeEvents, ev)
	}))

	bmv := NewBaseManaVector(ConsensusMana)

	// init vector to inputTime with pledged beforeBookingAmount
	bmv.SetMana(inputPledgeID1, NewManaBase(beforeBookingAmount[inputPledgeID1]))
	bmv.SetMana(inputPledgeID2, NewManaBase(beforeBookingAmount[inputPledgeID2]))
	bmv.SetMana(inputPledgeID3, NewManaBase(beforeBookingAmount[inputPledgeID3]))

	// drop all recorded updatedEvents
	updateEvents = []*UpdatedEvent{}

	// book mana with txInfo at txTime
	bmv.Book(txInfo)

	// expected nodeIDs to be called with each event
	updatedNodeIds := map[identity.ID]interface{}{
		txPledgeID:     0,
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}
	pledgedNodeIds := map[identity.ID]interface{}{
		txPledgeID: 0,
	}
	revokedNodeIds := map[identity.ID]interface{}{
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}

	// update triggered for the 3 nodes that mana was revoked from, and once for the pledged
	assert.Equal(t, 4, len(updateEvents))
	assert.Equal(t, 1, len(pledgeEvents))
	assert.Equal(t, 3, len(revokeEvents))

	for _, ev := range updateEvents {
		// has the right type
		assert.Equal(t, ConsensusMana, ev.ManaType)
		// base mana values are expected
		assert.Equal(t, afterBookingAmount[ev.NodeID], ev.NewMana.BaseValue())
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
	for _, ev := range pledgeEvents {
		assert.Equal(t, afterBookingAmount[ev.NodeID], ev.Amount)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, ConsensusMana, ev.ManaType)
		assert.Contains(t, pledgedNodeIds, ev.NodeID)
		delete(pledgedNodeIds, ev.NodeID)
	}
	assert.Empty(t, pledgedNodeIds)
	for _, ev := range revokeEvents {
		assert.Equal(t, beforeBookingAmount[ev.NodeID], ev.Amount)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, ConsensusMana, ev.ManaType)
		assert.Contains(t, revokedNodeIds, ev.NodeID)
		delete(revokedNodeIds, ev.NodeID)
	}
	assert.Empty(t, revokedNodeIds)
}

func TestConsensusBaseManaVector_BookEpoch(t *testing.T) {
	// hold information about which events triggered
	var (
		updateEvents []*UpdatedEvent
		revokeEvents []*RevokedEvent
		pledgeEvents []*PledgedEvent
	)
	nodeIds := map[identity.ID]types.Empty{
		inputPledgeID1: types.Void,
		inputPledgeID2: types.Void,
		inputPledgeID3: types.Void,
	}

	fmt.Println("all ids: ", inputPledgeID1.String(), ' ', inputPledgeID2.String(), ' ', inputPledgeID3.String())
	created, spent := prepareEpochDiffs()
	// when an event triggers, add it to the log
	Events.Updated.Hook(event.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))
	Events.Revoked.Hook(event.NewClosure(func(ev *RevokedEvent) {
		revokeEvents = append(revokeEvents, ev)
	}))
	Events.Pledged.Hook(event.NewClosure(func(ev *PledgedEvent) {
		pledgeEvents = append(pledgeEvents, ev)
	}))
	bmv := NewBaseManaVector(ConsensusMana)

	// init vector to inputTime with pledged beforeBookingAmount
	bmv.SetMana(inputPledgeID1, NewManaBase(beforeBookingAmount[inputPledgeID1]))
	bmv.SetMana(inputPledgeID2, NewManaBase(beforeBookingAmount[inputPledgeID2]))
	bmv.SetMana(inputPledgeID3, NewManaBase(beforeBookingAmount[inputPledgeID3]))

	bmv.BookEpoch(created, spent)

	// update triggered for the 3 nodes that mana was revoked from, and once for the pledged
	assert.Equal(t, 9, len(updateEvents))
	assert.Equal(t, 5, len(pledgeEvents))
	assert.Equal(t, 4, len(revokeEvents))

	latestUpdateEvent := make(map[identity.ID]*UpdatedEvent)
	for _, ev := range updateEvents {
		latestUpdateEvent[ev.NodeID] = ev
	}
	// check only the latest update event for each nodeID
	for _, ev := range latestUpdateEvent {
		// has the right type
		assert.Equal(t, ConsensusMana, ev.ManaType)
		// base mana values are expected
		assert.Equal(t, afterBookingEpochAmount[ev.NodeID], ev.NewMana.BaseValue())
		assert.Contains(t, nodeIds, ev.NodeID)
	}

	afterEventsAmount := make(map[identity.ID]float64)
	afterEventsAmount[inputPledgeID1] = beforeBookingAmount[inputPledgeID1]
	afterEventsAmount[inputPledgeID2] = beforeBookingAmount[inputPledgeID2]
	afterEventsAmount[inputPledgeID3] = beforeBookingAmount[inputPledgeID3]

	for i, ev := range revokeEvents {
		afterEventsAmount[epochSpentPledgeIDs[i]] -= ev.Amount
		assert.Equal(t, ConsensusMana, ev.ManaType)
		assert.Contains(t, nodeIds, ev.NodeID)
	}
	for i, ev := range pledgeEvents {
		afterEventsAmount[epochCreatedPledgeIDs[i]] += ev.Amount
		assert.Equal(t, ConsensusMana, ev.ManaType)
		assert.Contains(t, nodeIds, ev.NodeID)
	}
	// make sure pledge and revoke events balance changes are as expected
	for id := range afterBookingEpochAmount {
		assert.Equal(t, afterEventsAmount[id], afterBookingEpochAmount[id])
		m, _, err := bmv.GetMana(id)
		require.NoError(t, err)
		assert.Equal(t, afterBookingEpochAmount[id], m)
	}
}

func TestConsensusBaseManaVector_GetMana(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)
	randID := randNodeID()
	mana, _, err := bmv.GetMana(randID)
	assert.Equal(t, 0.0, mana)
	assert.Error(t, err)

	bmv.SetMana(randID, NewManaBase(0))
	mana, _, err = bmv.GetMana(randID)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, mana)

	bmv.SetMana(randID, NewManaBase(10.0))

	mana, _, err = bmv.GetMana(randID)
	assert.NoError(t, err)
	assert.Equal(t, 10.0, mana)
}

func TestConsensusBaseManaVector_ForEach(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)

	for i := 0; i < 10000; i++ {
		bmv.SetMana(randNodeID(), NewManaBase(1.0))
	}

	// fore each should iterate over all elements
	sum := 0.0
	bmv.ForEach(func(ID identity.ID, bm BaseMana) bool {
		sum += bm.BaseValue()
		return true
	})
	assert.Equal(t, 10000.0, sum)

	// for each should stop if false is returned from callback
	sum = 0.0
	bmv.ForEach(func(ID identity.ID, bm BaseMana) bool {
		if sum >= 5000.0 {
			return false
		}
		sum += bm.BaseValue()
		return true
	})

	assert.Equal(t, 5000.0, sum)
}

func TestConsensusBaseManaVector_GetManaMap(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)

	// empty vector returns empty map
	manaMap, _, err := bmv.GetManaMap()
	assert.NoError(t, err)
	assert.Empty(t, manaMap)

	nodeIDs := map[identity.ID]int{}

	for i := 0; i < 100; i++ {
		id := randNodeID()
		bmv.SetMana(id, NewManaBase(10.0))
		nodeIDs[id] = 0
	}

	manaMap, _, err = bmv.GetManaMap()
	assert.NoError(t, err)
	assert.Equal(t, 100, len(manaMap))
	for nodeID, mana := range manaMap {
		assert.Equal(t, 10.0, mana)
		assert.Contains(t, nodeIDs, nodeID)
		delete(nodeIDs, nodeID)
	}
	assert.Empty(t, nodeIDs)
}

func TestConsensusBaseManaVector_GetHighestManaNodes(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)

	nodeIDs := make([]identity.ID, 10)

	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], NewManaBase(float64(i)))
	}

	// requesting the top mana holder
	result, _, err := bmv.GetHighestManaNodes(1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.Equal(t, 9.0, result[0].Mana)

	// requesting top 3 mana holders
	result, _, err = bmv.GetHighestManaNodes(3)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, 9.0, result[0].Mana)
	for index, value := range result {
		if index < 2 {
			// it's greater than the next one
			assert.True(t, value.Mana > result[index+1].Mana)
		}
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}

	// requesting more, than there currently are in the vector
	result, _, err = bmv.GetHighestManaNodes(20)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	for index, value := range result {
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}
}

func TestConsensusBaseManaVector_GetHighestManaNodesFraction(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)

	nodeIDs := make([]identity.ID, 10)

	baseTime = time.Now()

	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], NewManaBase(float64(i)))
	}

	// requesting minus value
	result, _, err := bmv.GetHighestManaNodesFraction(-0.1)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.Equal(t, 9.0, result[0].Mana)

	// requesting the holders of top 10% of mana
	result, _, err = bmv.GetHighestManaNodesFraction(0.1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.Equal(t, 9.0, result[0].Mana)

	// requesting holders of top 50% of mana
	result, _, err = bmv.GetHighestManaNodesFraction(0.5)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, 9.0, result[0].Mana)
	for index, value := range result {
		if index < 2 {
			// it's greater than the next one
			assert.True(t, value.Mana > result[index+1].Mana)
		}
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}

	// requesting more, than there currently are in the vector
	result, _, err = bmv.GetHighestManaNodesFraction(1.1)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	for index, value := range result {
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}
}

func TestConsensusBaseManaVector_SetMana(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)
	nodeIDs := make([]identity.ID, 10)
	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], NewManaBase(float64(i)))
	}
	for i := 0; i < 10; i++ {
		assert.Equal(t, NewManaBase(float64(i)), bmv.(*ManaBaseVector).M.Vector[nodeIDs[i]])
	}
}

func TestConsensusBaseManaVector_ToPersistables(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)
	id1 := randNodeID()
	id2 := randNodeID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, NewManaBase(data[id1]))
	bmv.SetMana(id2, NewManaBase(data[id2]))

	persistables := bmv.ToPersistables()

	assert.Equal(t, 2, len(persistables))
	for _, p := range persistables {
		assert.Equal(t, p.ManaType(), ConsensusMana)
		assert.Equal(t, 1, len(p.BaseValues()))
		assert.Equal(t, data[p.NodeID()], p.BaseValues()[0])
		delete(data, p.NodeID())
	}
	assert.Equal(t, 0, len(data))
}

func TestConsensusBaseManaVector_FromPersistable(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		id := randNodeID()
		p := NewPersistableBaseMana(id, ConsensusMana, []float64{10}, []float64{100}, baseTime)
		bmv := NewBaseManaVector(ConsensusMana)
		assert.False(t, bmv.Has(id))
		err := bmv.FromPersistable(p)
		assert.NoError(t, err)
		assert.True(t, bmv.Has(id))
		assert.Equal(t, 1, bmv.Size())
		bmValue := bmv.(*ManaBaseVector).M.Vector[id]
		assert.Equal(t, 10.0, bmValue.BaseValue())
	})

	t.Run("CASE: Wrong number of base values", func(t *testing.T) {
		p := NewPersistableBaseMana(randNodeID(), ConsensusMana, []float64{0, 0}, []float64{0}, baseTime)

		bmv := NewBaseManaVector(ConsensusMana)

		err := bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has 2 base values instead of 1")
	})
}

func TestConsensusBaseManaVector_ToAndFromPersistable(t *testing.T) {
	bmv := NewBaseManaVector(ConsensusMana)
	id1 := randNodeID()
	id2 := randNodeID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, NewManaBase(data[id1]))
	bmv.SetMana(id2, NewManaBase(data[id2]))

	persistables := bmv.ToPersistables()

	var restoredBmv BaseManaVector
	restoredBmv = NewBaseManaVector(ConsensusMana)

	for _, p := range persistables {
		err := restoredBmv.FromPersistable(p)
		assert.NoError(t, err)
	}
	assert.Equal(t, bmv.(*ManaBaseVector).M.Vector, restoredBmv.(*ManaBaseVector).M.Vector)
}
