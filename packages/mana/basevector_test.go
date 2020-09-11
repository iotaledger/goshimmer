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
	// update event triggered 3 times for the 3 nodes
	assert.Equal(t, 3, len(updateEvents))
	assert.Equal(t, 0, len(pledgeEvents))
	assert.Equal(t, 0, len(revokeEvents))
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
	}
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
	// drop all recorded updatedEvents
	updateEvents = []*UpdatedEvent{}

	// book mana with txInfo at txTime (baseTime + 6 hours)
	bmv.BookMana(txInfo)

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
	}
	// TODO: finish

	bmv.UpdateAll(txTime.Add(time.Hour * 6))
}
