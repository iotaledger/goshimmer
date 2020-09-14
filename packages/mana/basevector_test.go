package mana

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
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
		TimeStamp:    txTime,
		TotalBalance: 10.0,
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
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: inputTime,
				Amount:    beforeBookingAmount[inputPledgeID2],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID2,
					ConsensusMana: inputPledgeID2,
				},
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: inputTime,
				Amount:    beforeBookingAmount[inputPledgeID3],
				PledgeID: map[Type]identity.ID{
					AccessMana:    inputPledgeID3,
					ConsensusMana: inputPledgeID3,
				},
			},
		},
	}
)

func randNodeID() identity.ID {
	return identity.GenerateIdentity().ID()
}

func TestNewBaseManaVector(t *testing.T) {
	bmvAccess := NewBaseManaVector(AccessMana)
	assert.Equal(t, AccessMana, bmvAccess.vectorType)
	assert.Equal(t, map[identity.ID]*BaseMana{}, bmvAccess.vector)

	bmvCons := NewBaseManaVector(ConsensusMana)
	assert.Equal(t, ConsensusMana, bmvCons.vectorType)
	assert.Equal(t, map[identity.ID]*BaseMana{}, bmvCons.vector)
}

func TestBaseManaVector_Type(t *testing.T) {
	bmv := NewBaseManaVector(AccessMana)
	vectorType := bmv.Type()
	assert.Equal(t, AccessMana, vectorType)
}

func TestBaseManaVector_Size(t *testing.T) {
	bmv := NewBaseManaVector(AccessMana)
	assert.Equal(t, 0, bmv.Size())

	for i := 0; i < 10; i++ {
		bmv.SetMana(randNodeID(), &BaseMana{
			BaseMana1:          float64(i),
			EffectiveBaseMana1: float64(i),
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		})
	}
	assert.Equal(t, 10, bmv.Size())
}

func TestBaseManaVector_BookMana(t *testing.T) {
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

	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)

	// init vector to inputTime with pledged beforeBookingAmount
	bmv.SetMana(inputPledgeID1, &BaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID1],
		BaseMana2:   beforeBookingAmount[inputPledgeID1],
		LastUpdated: inputTime,
	})
	bmv.SetMana(inputPledgeID2, &BaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID2],
		BaseMana2:   beforeBookingAmount[inputPledgeID2],
		LastUpdated: inputTime,
	})
	bmv.SetMana(inputPledgeID3, &BaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID3],
		BaseMana2:   beforeBookingAmount[inputPledgeID3],
		LastUpdated: inputTime,
	})

	// update to txTime - 6 hours. Effective base manas should converge to their asymptote.
	bmv.UpdateAll(baseTime)
	updatedNodeIds := map[identity.ID]interface{}{
		inputPledgeID1: 0,
		inputPledgeID2: 0,
		inputPledgeID3: 0,
	}
	for _, ev := range updateEvents {
		// has the right type
		assert.Equal(t, AccessMana, ev.Type)
		// has the right update time
		assert.Equal(t, baseTime, ev.NewMana.LastUpdated)
		// base mana values are expected
		assert.Equal(t, beforeBookingAmount[ev.NodeID], ev.NewMana.BaseMana1)
		assert.True(t, within(beforeBookingAmount[ev.NodeID], ev.NewMana.EffectiveBaseMana1))
		assert.True(t, within(0, ev.NewMana.BaseMana2))
		assert.True(t, within(0, ev.NewMana.EffectiveBaseMana2))
		// update triggered for expected nodes
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		// remove this one from the list of expected to make sure it was only called once
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
	// check the same for the content of the vector
	bmv.ForEach(func(ID identity.ID, bm *BaseMana) bool {
		// has the right update time
		assert.Equal(t, baseTime, bm.LastUpdated)
		// base mana values are expected
		assert.Equal(t, beforeBookingAmount[ID], bm.BaseMana1)
		assert.True(t, within(beforeBookingAmount[ID], bm.EffectiveBaseMana1))
		assert.True(t, within(0, bm.BaseMana2))
		assert.True(t, within(0, bm.EffectiveBaseMana2))
		return true
	})
	// update event triggered 3 times for the 3 nodes
	assert.Equal(t, 3, len(updateEvents))
	assert.Equal(t, 0, len(pledgeEvents))
	assert.Equal(t, 0, len(revokeEvents))
	// drop all recorded updatedEvents
	updateEvents = []*UpdatedEvent{}

	// book mana with txInfo at txTime (baseTime + 6 hours)
	bmv.BookMana(txInfo)

	// expected nodeIDs to be called with each event
	updatedNodeIds = map[identity.ID]interface{}{
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
		assert.Equal(t, AccessMana, ev.Type)
		// has the right update time
		assert.Equal(t, txTime, ev.NewMana.LastUpdated)
		// base mana values are expected
		assert.Equal(t, afterBookingAmount[ev.NodeID], ev.NewMana.BaseMana1)
		assert.True(t, within(beforeBookingAmount[ev.NodeID], ev.NewMana.EffectiveBaseMana1))
		assert.True(t, within(afterBookingAmount[ev.NodeID], ev.NewMana.BaseMana2))
		assert.True(t, within(0, ev.NewMana.EffectiveBaseMana2))
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
	for _, ev := range pledgeEvents {
		assert.Equal(t, afterBookingAmount[ev.NodeID], ev.AmountBM1)
		assert.InEpsilon(t, afterBookingAmount[ev.NodeID], ev.AmountBM2, eps)
		assert.Equal(t, txTime, ev.Time)
		assert.Equal(t, AccessMana, ev.Type)
		assert.Contains(t, pledgedNodeIds, ev.NodeID)
		delete(pledgedNodeIds, ev.NodeID)
	}
	assert.Empty(t, pledgedNodeIds)
	for _, ev := range revokeEvents {
		assert.Equal(t, beforeBookingAmount[ev.NodeID], ev.AmountBM1)
		assert.Equal(t, txTime, ev.Time)
		assert.Equal(t, AccessMana, ev.Type)
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
	err := bmv.UpdateAll(updateTime)
	assert.NoError(t, err)

	for _, ev := range updateEvents {
		// has the right update time
		assert.Equal(t, updateTime, ev.NewMana.LastUpdated)
		// base mana values are expected
		assert.Equal(t, afterBookingAmount[ev.NodeID], ev.NewMana.BaseMana1)
		if ev.NodeID == txPledgeID {
			assert.InEpsilon(t, afterBookingAmount[ev.NodeID]/2, ev.NewMana.EffectiveBaseMana1, eps)
			assert.InEpsilon(t, afterBookingAmount[ev.NodeID]/2, ev.NewMana.BaseMana2, eps)
			assert.InEpsilon(t, 3.465731, ev.NewMana.EffectiveBaseMana2, eps)
		} else {
			assert.InEpsilon(t, beforeBookingAmount[ev.NodeID]/2, ev.NewMana.EffectiveBaseMana1, eps)
			assert.InEpsilon(t, 1.0, 1+ev.NewMana.BaseMana2, eps)
			assert.InEpsilon(t, 1.0, 1+ev.NewMana.EffectiveBaseMana2, eps)
		}
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)

	// check the same for the content of the vector
	bmv.ForEach(func(ID identity.ID, bm *BaseMana) bool {
		// has the right update time
		assert.Equal(t, updateTime, bm.LastUpdated)
		// base mana values are expected
		assert.Equal(t, afterBookingAmount[ID], bm.BaseMana1)
		if ID == txPledgeID {
			assert.InEpsilon(t, afterBookingAmount[ID]/2, bm.EffectiveBaseMana1, eps)
			assert.InEpsilon(t, afterBookingAmount[ID]/2, bm.BaseMana2, eps)
			assert.InEpsilon(t, 3.465731, bm.EffectiveBaseMana2, eps)
		} else {
			assert.InEpsilon(t, beforeBookingAmount[ID]/2, bm.EffectiveBaseMana1, eps)
			assert.InEpsilon(t, 1.0, 1+bm.BaseMana2, eps)
			assert.InEpsilon(t, 1.0, 1+bm.EffectiveBaseMana2, eps)
		}
		return true
	})
}

func TestBaseManaVector_BookManaPanic(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)
	// init vector to inputTime with pledged beforeBookingAmount
	bmv.SetMana(inputPledgeID1, &BaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID1],
		BaseMana2:   beforeBookingAmount[inputPledgeID1],
		LastUpdated: inputTime,
	})
	bmv.SetMana(inputPledgeID2, &BaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID2],
		BaseMana2:   beforeBookingAmount[inputPledgeID2],
		LastUpdated: inputTime,
	})
	bmv.SetMana(inputPledgeID3, &BaseMana{
		// BM1 will be negative when revoking
		BaseMana1:   0,
		BaseMana2:   beforeBookingAmount[inputPledgeID3],
		LastUpdated: inputTime,
	})
	assert.Panics(t, func() { bmv.BookMana(txInfo) })
}

func TestBaseManaVector_Update(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)

	// hold information about which events triggered
	var updateEvents []*UpdatedEvent

	// when an event triggers, add it to the log
	Events().Updated.Attach(events.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))

	randID := randNodeID()
	// init vector to baseTime
	bmv.SetMana(randID, &BaseMana{
		BaseMana1:   10.0,
		BaseMana2:   10.0,
		LastUpdated: baseTime,
	})
	updateTime := baseTime.Add(time.Hour * 6)
	err := bmv.Update(randID, updateTime)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(updateEvents))
	ev := updateEvents[0]
	assert.Equal(t, randID, ev.NodeID)
	assert.Equal(t, AccessMana, ev.Type)
	assert.Equal(t, BaseMana{
		BaseMana1:   10.0,
		BaseMana2:   10.0,
		LastUpdated: baseTime},
		ev.OldMana)
	assert.Equal(t, 10.0, ev.NewMana.BaseMana1)
	assert.InEpsilon(t, 5, ev.NewMana.EffectiveBaseMana1, eps)
	assert.InEpsilon(t, 5, ev.NewMana.BaseMana2, eps)
	assert.InEpsilon(t, 3.465731, ev.NewMana.EffectiveBaseMana2, eps)
	assert.Equal(t, updateTime, ev.NewMana.LastUpdated)
}

func TestBaseManaVector_UpdateError(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)

	// hold information about which events triggered
	var updateEvents []*UpdatedEvent

	// when an event triggers, add it to the log
	Events().Updated.Attach(events.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))

	randID := randNodeID()
	updateTime := baseTime.Add(time.Hour * 6)

	// vector is empty ,but we want to update a non existing ID in it
	err := bmv.Update(randID, updateTime)
	assert.Error(t, err)
	assert.Equal(t, ErrNodeNotFoundInBaseManaVector, err)
	// no event triggered
	assert.Empty(t, updateEvents)

	// init vector to baseTime
	bmv.SetMana(randID, &BaseMana{
		BaseMana1:   10.0,
		BaseMana2:   10.0,
		LastUpdated: updateTime,
	})
	// vector update to baseTime + 6 hours already
	err = bmv.Update(randID, baseTime)
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyUpdated, err)
}

func TestBaseManaVector_UpdateAll(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)

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
	bmv.SetMana(inputPledgeID1, &BaseMana{
		LastUpdated: baseTime,
	})
	bmv.SetMana(inputPledgeID2, &BaseMana{
		LastUpdated: baseTime,
	})
	bmv.SetMana(inputPledgeID3, &BaseMana{
		LastUpdated: baseTime,
	})

	updateTime := baseTime.Add(time.Hour)
	bmv.UpdateAll(updateTime)

	for _, mana := range bmv.vector {
		assert.Equal(t, updateTime, mana.LastUpdated)
	}

	assert.Equal(t, 3, len(updateEvents))
	for _, ev := range updateEvents {
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
}

func TestBaseManaVector_GetWeightedMana(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)
	randID := randNodeID()
	mana, err := bmv.GetWeightedMana(randID, 0.5)
	assert.Equal(t, 0.0, mana)
	assert.Error(t, err)

	bmv.SetMana(randID, &BaseMana{})
	mana, err = bmv.GetWeightedMana(randID, -0.5)
	assert.Equal(t, 0.0, mana)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidWeightParameter, err)
	mana, err = bmv.GetWeightedMana(randID, 1.5)
	assert.Equal(t, 0.0, mana)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidWeightParameter, err)

	bmv.SetMana(randID, &BaseMana{
		BaseMana1:          10.0,
		EffectiveBaseMana1: 10.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 1.0,
		LastUpdated:        time.Now(),
	})

	mana, err = bmv.GetWeightedMana(randID, 0.5)
	assert.NoError(t, err)
	assert.InEpsilon(t, 5.5, mana, eps)
	mana, err = bmv.GetWeightedMana(randID, 1)
	assert.NoError(t, err)
	assert.InEpsilon(t, 10.0, mana, eps)
	mana, err = bmv.GetWeightedMana(randID, 0)
	assert.NoError(t, err)
	assert.InEpsilon(t, 1.0, mana, eps)
}

func TestBaseManaVector_GetMana(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)
	randID := randNodeID()

	bmv.SetMana(randID, &BaseMana{
		BaseMana1:          10.0,
		EffectiveBaseMana1: 10.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 1.0,
		LastUpdated:        time.Now(),
	})

	mana, err := bmv.GetMana(randID)
	assert.NoError(t, err)
	assert.InEpsilon(t, 5.5, mana, eps)
}

func TestBaseManaVector_ForEach(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)

	for i := 0; i < 10000; i++ {
		bmv.SetMana(randNodeID(), &BaseMana{BaseMana1: 1.0})
	}

	// fore each should iterate over all elements
	sum := 0.0
	bmv.ForEach(func(ID identity.ID, bm *BaseMana) bool {
		sum += bm.BaseMana1
		return true
	})
	assert.Equal(t, 10000.0, sum)

	// for each should stop if false is returned from callback
	sum = 0.0
	bmv.ForEach(func(ID identity.ID, bm *BaseMana) bool {
		if sum >= 5000.0 {
			return false
		}
		sum += bm.BaseMana1
		return true
	})

	assert.Equal(t, 5000.0, sum)
}

func TestBaseManaVector_GetManaMap(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)

	// empty vector returns empty map
	manaMap := bmv.GetManaMap()
	assert.Empty(t, manaMap)

	now := time.Now()
	nodeIDs := map[identity.ID]int{}

	for i := 0; i < 100; i++ {
		id := randNodeID()
		bmv.SetMana(id, &BaseMana{
			BaseMana1:          10.0,
			EffectiveBaseMana1: 10.0,
			BaseMana2:          1.0,
			EffectiveBaseMana2: 1.0,
			LastUpdated:        now,
		})
		nodeIDs[id] = 0
	}

	manaMap = bmv.GetManaMap()
	assert.Equal(t, 100, len(manaMap))
	for nodeID, mana := range manaMap {
		assert.InEpsilon(t, 5.5, mana, eps)
		assert.Contains(t, nodeIDs, nodeID)
		delete(nodeIDs, nodeID)
	}
	assert.Empty(t, nodeIDs)
}

func TestBaseManaVector_GetHighestManaNodes(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)

	var nodeIDs = make([]identity.ID, 10)

	baseTime = time.Now()

	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &BaseMana{
			BaseMana1:          float64(i),
			EffectiveBaseMana1: float64(i),
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		})
	}

	// requesting zero highest mana nodes
	result := bmv.GetHighestManaNodes(0)
	assert.Empty(t, result)

	// requesting the top mana holder
	result = bmv.GetHighestManaNodes(1)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.InEpsilon(t, 9.0, result[0].Mana, eps)

	// requesting top 3 mana holders
	result = bmv.GetHighestManaNodes(3)
	assert.Equal(t, 3, len(result))
	assert.InEpsilon(t, 9.0, result[0].Mana, eps)
	for index, value := range result {
		if index < 2 {
			// it's greater than the next one
			assert.True(t, value.Mana > result[index+1].Mana)
		}
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}

	// requesting more, than there currently are in the vector
	result = bmv.GetHighestManaNodes(20)
	assert.Equal(t, 10, len(result))
	for index, value := range result {
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}
}

func TestBaseManaVector_SetMana(t *testing.T) {
	// let's use an Access Base Mana Vector
	bmv := NewBaseManaVector(AccessMana)
	var nodeIDs = make([]identity.ID, 10)
	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &BaseMana{
			BaseMana1:          float64(i),
			EffectiveBaseMana1: float64(i),
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		})
	}
	for i := 0; i < 10; i++ {
		assert.Equal(t, BaseMana{
			BaseMana1:          float64(i),
			EffectiveBaseMana1: float64(i),
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		}, *bmv.vector[nodeIDs[i]])
	}
}
