package mana

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

var (
	baseTime            = time.Now()
	inputTime           = baseTime.Add(time.Hour * -200)
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
				// funds have been sitting here for couple days...
				TimeStamp: inputTime,
				Amount:    beforeBookingAmount[inputPledgeID1],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID1,
					ConsensusMana: inputPledgeID1,
				},
				InputID: ledgerstate.OutputID{1},
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: inputTime,
				Amount:    beforeBookingAmount[inputPledgeID2],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID2,
					ConsensusMana: inputPledgeID2,
				},
				InputID: ledgerstate.OutputID{2},
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: inputTime,
				Amount:    beforeBookingAmount[inputPledgeID3],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID3,
					ConsensusMana: inputPledgeID3,
				},
				InputID: ledgerstate.OutputID{3},
			},
		},
	}
)

func randNodeID() identity.ID {
	return identity.GenerateIdentity().ID()
}

func TestNewBaseManaVector_Access(t *testing.T) {
	bmvAccess, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	assert.Equal(t, AccessMana, bmvAccess.Type())
	assert.Equal(t, map[identity.ID]*AccessBaseMana{}, bmvAccess.(*AccessBaseManaVector).vector)
}

func TestAccessBaseManaVector_Type(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	vectorType := bmv.Type()
	assert.Equal(t, AccessMana, vectorType)
}

func TestAccessBaseManaVector_Size(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	assert.Equal(t, 0, bmv.Size())

	for i := 0; i < 10; i++ {
		bmv.SetMana(randNodeID(), &AccessBaseMana{
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		})
	}
	assert.Equal(t, 10, bmv.Size())
}

func TestAccessBaseManaVector_Has(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	randID := randNodeID()

	has := bmv.Has(randID)
	assert.False(t, has)

	bmv.SetMana(randID, &AccessBaseMana{
		BaseMana2:          0,
		EffectiveBaseMana2: 0,
		LastUpdated:        time.Time{},
	})
	has = bmv.Has(randID)
	assert.True(t, has)
}

func TestAccessBaseManaVector_Book(t *testing.T) {
	// hold information about which events triggered
	var (
		updateEvents []*UpdatedEvent
		revokeEvents []*RevokedEvent
		pledgeEvents []*PledgedEvent
	)

	// when an event triggers, add it to the log
	Events().Updated.Attach(events.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))
	Events().Revoked.Attach(events.NewClosure(func(ev *RevokedEvent) {
		revokeEvents = append(revokeEvents, ev)
	}))
	Events().Pledged.Attach(events.NewClosure(func(ev *PledgedEvent) {
		pledgeEvents = append(pledgeEvents, ev)
	}))

	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	// init vector to inputTime with pledged beforeBookingAmount
	bmv.SetMana(inputPledgeID1, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID1],
		LastUpdated: inputTime,
	})
	bmv.SetMana(inputPledgeID2, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID2],
		LastUpdated: inputTime,
	})
	bmv.SetMana(inputPledgeID3, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID3],
		LastUpdated: inputTime,
	})

	// update to txTime - 6 hours. Effective base manas should converge to their asymptote.
	err = bmv.UpdateAll(baseTime)
	assert.NoError(t, err)
	updatedNodeIds := map[identity.ID]interface{}{
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}
	for _, ev := range updateEvents {
		// has the right type
		assert.Equal(t, AccessMana, ev.ManaType)
		// has the right update time
		assert.Equal(t, baseTime, ev.NewMana.LastUpdate())
		// base mana values are expected
		assert.InDelta(t, 0, ev.NewMana.BaseValue(), delta)
		assert.InDelta(t, 0, ev.NewMana.EffectiveValue(), delta)
		// update triggered for expected nodes
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		// remove this one from the list of expected to make sure it was only called once
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
	// check the same for the content of the vector
	bmv.ForEach(func(ID identity.ID, bm BaseMana) bool {
		// has the right update time
		assert.Equal(t, baseTime, bm.LastUpdate())
		// base mana values are expected
		assert.InDelta(t, 0, bm.BaseValue(), delta)
		assert.InDelta(t, 0, bm.EffectiveValue(), delta)
		return true
	})
	// update event triggered 3 times for the 3 nodes
	assert.Equal(t, 3, len(updateEvents))
	assert.Equal(t, 0, len(pledgeEvents))
	assert.Equal(t, 0, len(revokeEvents))
	// drop all recorded updatedEvents
	updateEvents = []*UpdatedEvent{}

	// book mana with txInfo at txTime (baseTime + 6 hours)
	bmv.Book(txInfo)

	// expected nodeIDs to be called with each event
	updatedNodeIds = map[identity.ID]interface{}{
		txPledgeID: 0,
	}
	pledgedNodeIds := map[identity.ID]interface{}{
		txPledgeID: 0,
	}
	revokedNodeIds := map[identity.ID]interface{}{}

	// update triggered and once for the pledged
	assert.Equal(t, 1, len(updateEvents))
	assert.Equal(t, 1, len(pledgeEvents))
	assert.Equal(t, 0, len(revokeEvents))

	for _, ev := range updateEvents {
		// has the right type
		assert.Equal(t, AccessMana, ev.ManaType)
		// has the right update time
		assert.Equal(t, txTime, ev.NewMana.LastUpdate())
		// base mana values are expected
		assert.InDelta(t, afterBookingAmount[ev.NodeID], ev.NewMana.BaseValue(), delta)
		assert.InDelta(t, 0, ev.NewMana.EffectiveValue(), delta)
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
	for _, ev := range pledgeEvents {
		assert.InDelta(t, afterBookingAmount[ev.NodeID], ev.Amount, delta)
		assert.Equal(t, txTime, ev.Time)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, AccessMana, ev.ManaType)
		assert.Contains(t, pledgedNodeIds, ev.NodeID)
		delete(pledgedNodeIds, ev.NodeID)
	}
	assert.Empty(t, pledgedNodeIds)
	for _, ev := range revokeEvents {
		assert.Equal(t, beforeBookingAmount[ev.NodeID], ev.Amount)
		assert.Equal(t, txTime, ev.Time)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, AccessMana, ev.ManaType)
		assert.Contains(t, revokedNodeIds, ev.NodeID)
		delete(revokedNodeIds, ev.NodeID)
	}
	assert.Empty(t, revokedNodeIds)

	// drop all recorded updatedEvents
	updateEvents = []*UpdatedEvent{}
	// expected nodeIDs to be called with each event
	updatedNodeIds = map[identity.ID]interface{}{
		txPledgeID:     0,
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}

	updateTime := txTime.Add(time.Hour * 6)
	err = bmv.UpdateAll(updateTime)
	assert.NoError(t, err)

	for _, ev := range updateEvents {
		// has the right update time
		assert.Equal(t, updateTime, ev.NewMana.LastUpdate())
		// base mana values are expected
		if ev.NodeID == txPledgeID {
			assert.InDelta(t, afterBookingAmount[ev.NodeID]/2, ev.NewMana.BaseValue(), delta)
			assert.InDelta(t, 3.465731, ev.NewMana.EffectiveValue(), delta)
		} else {
			assert.InDelta(t, 1.0, 1+ev.NewMana.BaseValue(), delta)
			assert.InDelta(t, 1.0, 1+ev.NewMana.EffectiveValue(), delta)
		}
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)

	// check the same for the content of the vector
	bmv.ForEach(func(ID identity.ID, bm BaseMana) bool {
		// has the right update time
		assert.Equal(t, updateTime, bm.LastUpdate())
		// base mana values are expected
		if ID == txPledgeID {
			assert.InDelta(t, afterBookingAmount[ID]/2, bm.BaseValue(), delta)
			assert.InDelta(t, 3.465731, bm.EffectiveValue(), delta)
		} else {
			assert.InDelta(t, 1.0, 1+bm.BaseValue(), delta)
			assert.InDelta(t, 1.0, 1+bm.EffectiveValue(), delta)
		}
		return true
	})
}

func TestAccessBaseManaVector_Update(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	// hold information about which events triggered
	var updateEvents []*UpdatedEvent

	// when an event triggers, add it to the log
	Events().Updated.Attach(events.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))

	randID := randNodeID()
	// init vector to baseTime
	bmv.SetMana(randID, &AccessBaseMana{
		BaseMana2:   10.0,
		LastUpdated: baseTime,
	})
	updateTime := baseTime.Add(time.Hour * 6)
	err = bmv.Update(randID, updateTime)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(updateEvents))
	ev := updateEvents[0]
	assert.Equal(t, randID, ev.NodeID)
	assert.Equal(t, AccessMana, ev.ManaType)
	assert.Equal(t, &AccessBaseMana{
		BaseMana2:   10.0,
		LastUpdated: baseTime,
	},
		ev.OldMana)
	assert.InDelta(t, 5, ev.NewMana.BaseValue(), delta)
	assert.InDelta(t, 3.465731, ev.NewMana.EffectiveValue(), delta)
	assert.Equal(t, updateTime, ev.NewMana.LastUpdate())
}

func TestAccessBaseManaVector_UpdateError(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	// hold information about which events triggered
	var updateEvents []*UpdatedEvent

	// when an event triggers, add it to the log
	Events().Updated.Attach(events.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))

	randID := randNodeID()
	updateTime := baseTime.Add(time.Hour * 6)

	// vector is empty ,but we want to update a non existing ID in it
	err = bmv.Update(randID, updateTime)
	assert.Error(t, err)
	assert.Equal(t, ErrNodeNotFoundInBaseManaVector, err)
	// no event triggered
	assert.Empty(t, updateEvents)

	// init vector to baseTime
	bmv.SetMana(randID, &AccessBaseMana{
		BaseMana2:   10.0,
		LastUpdated: updateTime,
	})
	// vector update to baseTime + 6 hours already
	err = bmv.Update(randID, baseTime)
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyUpdated, err)
}

func TestAccessBaseManaVector_UpdateAll(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	// hold information about which events triggered
	var updateEvents []*UpdatedEvent

	// when an event triggers, add it to the log
	Events().Updated.Attach(events.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))

	updatedNodeIds := map[identity.ID]interface{}{
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}

	// init vector (values are not important)
	bmv.SetMana(inputPledgeID1, &AccessBaseMana{
		LastUpdated: baseTime,
	})
	bmv.SetMana(inputPledgeID2, &AccessBaseMana{
		LastUpdated: baseTime,
	})
	bmv.SetMana(inputPledgeID3, &AccessBaseMana{
		LastUpdated: baseTime,
	})

	updateTime := baseTime.Add(time.Hour)
	err = bmv.UpdateAll(updateTime)
	assert.NoError(t, err)

	for _, mana := range bmv.(*AccessBaseManaVector).vector {
		assert.Equal(t, updateTime, mana.LastUpdated)
	}

	assert.Equal(t, 3, len(updateEvents))
	for _, ev := range updateEvents {
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
}

func TestAccessBaseManaVector_GetMana(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	randID := randNodeID()
	mana, _, err := bmv.GetMana(randID)
	assert.Equal(t, 0.0, mana)
	assert.NoError(t, err)
	bmv.SetMana(randID, &AccessBaseMana{})
	mana, _, err = bmv.GetMana(randID)
	assert.Equal(t, 0.0, mana)
	assert.NoError(t, err)
	now := time.Now()
	bmv.SetMana(randID, &AccessBaseMana{
		BaseMana2:          1.0,
		EffectiveBaseMana2: 1.0,
		LastUpdated:        now,
	})

	mana, _, err = bmv.GetMana(randID, now)
	assert.NoError(t, err)
	assert.InDelta(t, 1.0, mana, delta)
}

func TestAccessBaseManaVector_ForEach(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	for i := 0; i < 10000; i++ {
		bmv.SetMana(randNodeID(), &AccessBaseMana{BaseMana2: 1.0})
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

func TestAccessBaseManaVector_GetManaMap(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	// empty vector returns empty map
	manaMap, _, err := bmv.GetManaMap()
	assert.NoError(t, err)
	assert.Empty(t, manaMap)

	now := time.Now()
	nodeIDs := map[identity.ID]int{}

	for i := 0; i < 100; i++ {
		id := randNodeID()
		bmv.SetMana(id, &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 1.0,
			LastUpdated:        now,
		})
		nodeIDs[id] = 0
	}

	manaMap, _, err = bmv.GetManaMap(now)
	assert.NoError(t, err)
	assert.Equal(t, 100, len(manaMap))
	for nodeID, mana := range manaMap {
		assert.InDelta(t, 1.0, mana, delta)
		assert.Contains(t, nodeIDs, nodeID)
		delete(nodeIDs, nodeID)
	}
	assert.Empty(t, nodeIDs)
}

func TestAccessBaseManaVector_GetHighestManaNodes(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	nodeIDs := make([]identity.ID, 10)

	baseTime = time.Now()

	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &AccessBaseMana{
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		})
	}

	// requesting the top mana holder
	result, _, err := bmv.GetHighestManaNodes(1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.InDelta(t, 9.0, result[0].Mana, delta)

	// requesting top 3 mana holders
	result, _, err = bmv.GetHighestManaNodes(3)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result))
	assert.InDelta(t, 9.0, result[0].Mana, delta)
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
	for index, value := range result[:8] {
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}
}

func TestAccessBaseManaVector_GetHighestManaNodesFraction(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	nodeIDs := make([]identity.ID, 10)

	baseTime = time.Now()

	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &AccessBaseMana{
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		})
	}

	// requesting minus value
	result, _, err := bmv.GetHighestManaNodesFraction(-0.1)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.InDelta(t, 9.0, result[0].Mana, delta)

	// requesting the holders of top 5% of mana
	result, _, err = bmv.GetHighestManaNodesFraction(0.1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.InDelta(t, 9.0, result[0].Mana, delta)

	// requesting holders of top 50% of mana
	result, _, err = bmv.GetHighestManaNodesFraction(0.5)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result))
	assert.InDelta(t, 9.0, result[0].Mana, delta)
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
	for index, value := range result[:8] {
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}
}

func TestAccessBaseManaVector_SetMana(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	nodeIDs := make([]identity.ID, 10)
	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &AccessBaseMana{
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		})
	}
	for i := 0; i < 10; i++ {
		assert.Equal(t, &AccessBaseMana{
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		}, bmv.(*AccessBaseManaVector).vector[nodeIDs[i]])
	}
}

func TestAccessBaseManaVector_ToPersistables(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	id1 := randNodeID()
	id2 := randNodeID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, &AccessBaseMana{
		BaseMana2:          data[id1],
		EffectiveBaseMana2: data[id1],
		LastUpdated:        baseTime,
	})
	bmv.SetMana(id2, &AccessBaseMana{
		BaseMana2:          data[id2],
		EffectiveBaseMana2: data[id2],
		LastUpdated:        baseTime,
	})

	persistables := bmv.ToPersistables()

	assert.Equal(t, 2, len(persistables))
	for _, p := range persistables {
		assert.Equal(t, p.ManaType, AccessMana)
		assert.Equal(t, p.LastUpdated, baseTime)
		assert.Equal(t, 1, len(p.BaseValues))
		assert.Equal(t, 1, len(p.EffectiveValues))
		assert.Equal(t, data[p.NodeID], p.BaseValues[0])
		assert.Equal(t, data[p.NodeID], p.EffectiveValues[0])
		delete(data, p.NodeID)
	}
	assert.Equal(t, 0, len(data))
}

func TestAccessBaseManaVector_FromPersistable(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		id := randNodeID()
		p := &PersistableBaseMana{
			ManaType:        AccessMana,
			BaseValues:      []float64{10},
			EffectiveValues: []float64{100},
			LastUpdated:     baseTime,
			NodeID:          id,
		}

		bmv, err := NewBaseManaVector(AccessMana)
		assert.NoError(t, err)
		assert.False(t, bmv.Has(id))
		err = bmv.FromPersistable(p)
		assert.NoError(t, err)
		assert.True(t, bmv.Has(id))
		assert.Equal(t, 1, bmv.Size())
		bmValue := bmv.(*AccessBaseManaVector).vector[id]
		assert.Equal(t, 10.0, bmValue.BaseValue())
		assert.Equal(t, 100.0, bmValue.EffectiveValue())
		assert.Equal(t, baseTime, bmValue.LastUpdate())
	})

	t.Run("CASE: Wrong type", func(t *testing.T) {
		p := &PersistableBaseMana{
			ManaType:        ConsensusMana,
			BaseValues:      []float64{0},
			EffectiveValues: []float64{0},
			LastUpdated:     baseTime,
			NodeID:          randNodeID(),
		}

		bmv, err := NewBaseManaVector(AccessMana)
		assert.NoError(t, err)

		err = bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has type Consensus instead of Access")
	})

	t.Run("CASE: Wrong number of base values", func(t *testing.T) {
		p := &PersistableBaseMana{
			ManaType:        AccessMana,
			BaseValues:      []float64{0, 0},
			EffectiveValues: []float64{0},
			LastUpdated:     baseTime,
			NodeID:          randNodeID(),
		}

		bmv, err := NewBaseManaVector(AccessMana)
		assert.NoError(t, err)

		err = bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has 2 base values instead of 1")
	})

	t.Run("CASE: Wrong number of effective values", func(t *testing.T) {
		p := &PersistableBaseMana{
			ManaType:        AccessMana,
			BaseValues:      []float64{0},
			EffectiveValues: []float64{0, 0},
			LastUpdated:     baseTime,
			NodeID:          randNodeID(),
		}

		bmv, err := NewBaseManaVector(AccessMana)
		assert.NoError(t, err)

		err = bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has 2 effective values instead of 1")
	})
}

func TestAccessBaseManaVector_ToAndFromPersistable(t *testing.T) {
	bmv, err := NewBaseManaVector(AccessMana)
	assert.NoError(t, err)
	id1 := randNodeID()
	id2 := randNodeID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, &AccessBaseMana{
		BaseMana2:          data[id1],
		EffectiveBaseMana2: data[id1],
		LastUpdated:        baseTime,
	})
	bmv.SetMana(id2, &AccessBaseMana{
		BaseMana2:          data[id2],
		EffectiveBaseMana2: data[id2],
		LastUpdated:        baseTime,
	})

	persistables := bmv.ToPersistables()

	var restoredBmv BaseManaVector
	restoredBmv, err = NewBaseManaVector(AccessMana)
	assert.NoError(t, err)

	for _, p := range persistables {
		err = restoredBmv.FromPersistable(p)
		assert.NoError(t, err)
	}
	assert.Equal(t, bmv.(*AccessBaseManaVector).vector, restoredBmv.(*AccessBaseManaVector).vector)
}
