package mana

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"golang.org/x/xerrors"
)

func TestNewResearchBaseManaVector(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
		assert.NoError(t, err)
		assert.Equal(t, WeightedMana, bmv.Type())
		assert.Equal(t, map[identity.ID]*WeightedBaseMana{}, bmv.(*WeightedBaseManaVector).vector)
	})

	t.Run("CASE: Wrong type", func(t *testing.T) {
		_, err := NewResearchBaseManaVector(AccessMana, AccessMana, Mixed)
		assert.Error(t, err)
		assert.True(t, xerrors.Is(err, ErrUnknownManaType))
	})

	t.Run("CASE: Wrong target type", func(t *testing.T) {
		_, err := NewResearchBaseManaVector(WeightedMana, WeightedMana, Mixed)
		assert.Error(t, err)
		assert.True(t, xerrors.Is(err, ErrInvalidTargetManaType))
	})

	t.Run("CASE: Invalid weight parameter", func(t *testing.T) {
		_, err := NewResearchBaseManaVector(WeightedMana, ConsensusMana, 5.0)
		assert.Error(t, err)
		assert.True(t, xerrors.Is(err, ErrInvalidWeightParameter))
	})
}

func TestWeightedBaseManaVector_Type(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	vectorType := bmv.Type()
	assert.Equal(t, WeightedMana, vectorType)
}

func TestWeightedBaseManaVector_Target(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	target := bmv.(*WeightedBaseManaVector).Target()
	assert.Equal(t, AccessMana, target)
}

func TestWeightedBaseManaVector_Size(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	assert.Equal(t, 0, bmv.Size())

	for i := 0; i < 10; i++ {
		wm := NewWeightedMana(Mixed)
		wm.mana1 = &ConsensusBaseMana{
			BaseMana1:          float64(i),
			EffectiveBaseMana1: float64(i),
			LastUpdated:        baseTime,
		}
		wm.mana2 = &AccessBaseMana{
			BaseMana2:          float64(i),
			EffectiveBaseMana2: float64(i),
			LastUpdated:        baseTime,
		}
		bmv.SetMana(randNodeID(), wm)
	}
	assert.Equal(t, 10, bmv.Size())
}

func TestWeightedBaseManaVector_Has(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	randID := randNodeID()

	has := bmv.Has(randID)
	assert.False(t, has)

	bmv.SetMana(randID, NewWeightedMana(Mixed))
	has = bmv.Has(randID)
	assert.True(t, has)
}

func TestWeightedBaseManaVector_Book(t *testing.T) {
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

	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)

	// init vector to inputTime with pledged beforeBookingAmount
	bmv.(*WeightedBaseManaVector).SetMana1(inputPledgeID1, &ConsensusBaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID1],
		LastUpdated: inputTime,
	})
	bmv.(*WeightedBaseManaVector).SetMana2(inputPledgeID1, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID1],
		LastUpdated: inputTime,
	})
	// init vector to inputTime with pledged beforeBookingAmount
	bmv.(*WeightedBaseManaVector).SetMana1(inputPledgeID2, &ConsensusBaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID2],
		LastUpdated: inputTime,
	})
	bmv.(*WeightedBaseManaVector).SetMana2(inputPledgeID2, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID2],
		LastUpdated: inputTime,
	})
	// init vector to inputTime with pledged beforeBookingAmount
	bmv.(*WeightedBaseManaVector).SetMana1(inputPledgeID3, &ConsensusBaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID3],
		LastUpdated: inputTime,
	})
	bmv.(*WeightedBaseManaVector).SetMana2(inputPledgeID3, &AccessBaseMana{
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
		assert.Equal(t, WeightedMana, ev.ManaType)
		// has the right update time
		assert.Equal(t, baseTime, ev.NewMana.LastUpdate())
		// base mana values are expected
		assert.Equal(t, beforeBookingAmount[ev.NodeID], ev.NewMana.(*WeightedBaseMana).mana1.BaseMana1)
		assert.InDelta(t, beforeBookingAmount[ev.NodeID], ev.NewMana.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
		assert.InDelta(t, 0, ev.NewMana.(*WeightedBaseMana).mana2.BaseMana2, delta)
		assert.InDelta(t, 0, ev.NewMana.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
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
		assert.Equal(t, beforeBookingAmount[ID], bm.(*WeightedBaseMana).mana1.BaseMana1)
		assert.InDelta(t, beforeBookingAmount[ID], bm.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
		assert.InDelta(t, 0, bm.(*WeightedBaseMana).mana2.BaseMana2, delta)
		assert.InDelta(t, 0, bm.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
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
		assert.Equal(t, WeightedMana, ev.ManaType)
		// has the right update time
		assert.Equal(t, txTime, ev.NewMana.LastUpdate())
		// base mana values are expected
		assert.Equal(t, afterBookingAmount[ev.NodeID], ev.NewMana.(*WeightedBaseMana).mana1.BaseMana1)
		assert.InDelta(t, beforeBookingAmount[ev.NodeID], ev.NewMana.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
		assert.InDelta(t, afterBookingAmount[ev.NodeID], ev.NewMana.(*WeightedBaseMana).mana2.BaseMana2, delta)
		assert.InDelta(t, 0, ev.NewMana.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
	for _, ev := range pledgeEvents {
		assert.InDelta(t, afterBookingAmount[ev.NodeID], ev.Amount, delta)
		assert.Equal(t, txTime, ev.Time)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, WeightedMana, ev.ManaType)
		assert.Contains(t, pledgedNodeIds, ev.NodeID)
		delete(pledgedNodeIds, ev.NodeID)
	}
	assert.Empty(t, pledgedNodeIds)
	for _, ev := range revokeEvents {
		assert.Equal(t, beforeBookingAmount[ev.NodeID], ev.Amount)
		assert.Equal(t, txTime, ev.Time)
		assert.Equal(t, txInfo.TransactionID, ev.TransactionID)
		assert.Equal(t, WeightedMana, ev.ManaType)
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
		assert.Equal(t, afterBookingAmount[ev.NodeID], ev.NewMana.(*WeightedBaseMana).mana1.BaseMana1)
		if ev.NodeID == txPledgeID {
			assert.InDelta(t, afterBookingAmount[ev.NodeID]/2, ev.NewMana.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
			assert.InDelta(t, afterBookingAmount[ev.NodeID]/2, ev.NewMana.(*WeightedBaseMana).mana2.BaseMana2, delta)
			assert.InDelta(t, 3.465731, ev.NewMana.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
		} else {
			assert.InDelta(t, beforeBookingAmount[ev.NodeID]/2, ev.NewMana.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
			assert.InDelta(t, 1.0, 1+ev.NewMana.(*WeightedBaseMana).mana2.BaseMana2, delta)
			assert.InDelta(t, 1.0, 1+ev.NewMana.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
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
		assert.Equal(t, afterBookingAmount[ID], bm.(*WeightedBaseMana).mana1.BaseMana1)
		if ID == txPledgeID {
			assert.InDelta(t, afterBookingAmount[ID]/2, bm.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
			assert.InDelta(t, afterBookingAmount[ID]/2, bm.(*WeightedBaseMana).mana2.BaseMana2, delta)
			assert.InDelta(t, 3.465731, bm.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
		} else {
			assert.InDelta(t, beforeBookingAmount[ID]/2, bm.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
			assert.InDelta(t, 1.0, 1+bm.(*WeightedBaseMana).mana2.BaseMana2, delta)
			assert.InDelta(t, 1.0, 1+bm.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
		}
		return true
	})
}

func TestWeightedBaseManaVector_BookManaPanic(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	// init vector to inputTime with pledged beforeBookingAmount
	bmv.(*WeightedBaseManaVector).SetMana1(inputPledgeID1, &ConsensusBaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID1],
		LastUpdated: inputTime,
	})
	bmv.(*WeightedBaseManaVector).SetMana2(inputPledgeID1, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID1],
		LastUpdated: inputTime,
	})
	// init vector to inputTime with pledged beforeBookingAmount
	bmv.(*WeightedBaseManaVector).SetMana1(inputPledgeID2, &ConsensusBaseMana{
		BaseMana1:   beforeBookingAmount[inputPledgeID2],
		LastUpdated: inputTime,
	})
	bmv.(*WeightedBaseManaVector).SetMana2(inputPledgeID2, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID2],
		LastUpdated: inputTime,
	})
	// init vector to inputTime with pledged beforeBookingAmount
	bmv.(*WeightedBaseManaVector).SetMana1(inputPledgeID3, &ConsensusBaseMana{
		BaseMana1:   0,
		LastUpdated: inputTime,
	})
	bmv.(*WeightedBaseManaVector).SetMana2(inputPledgeID3, &AccessBaseMana{
		BaseMana2:   beforeBookingAmount[inputPledgeID3],
		LastUpdated: inputTime,
	})
	assert.Panics(t, func() { bmv.Book(txInfo) })
}

func TestWeightedBaseManaVector_Update(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)

	// hold information about which events triggered
	var updateEvents []*UpdatedEvent

	// when an event triggers, add it to the log
	Events().Updated.Attach(events.NewClosure(func(ev *UpdatedEvent) {
		updateEvents = append(updateEvents, ev)
	}))

	randID := randNodeID()
	// init vector to baseTime
	bmv.(*WeightedBaseManaVector).SetMana1(randID, &ConsensusBaseMana{
		BaseMana1:   10,
		LastUpdated: baseTime,
	})
	bmv.(*WeightedBaseManaVector).SetMana2(randID, &AccessBaseMana{
		BaseMana2:   10,
		LastUpdated: baseTime,
	})
	updateTime := baseTime.Add(time.Hour * 6)
	err = bmv.Update(randID, updateTime)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(updateEvents))
	ev := updateEvents[0]
	assert.Equal(t, randID, ev.NodeID)
	assert.Equal(t, WeightedMana, ev.ManaType)
	assert.Equal(t, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:   10,
			LastUpdated: baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:   10,
			LastUpdated: baseTime,
		},
		weight: Mixed,
	},
		ev.OldMana)
	assert.Equal(t, 10.0, ev.NewMana.(*WeightedBaseMana).mana1.BaseMana1)
	assert.InDelta(t, 5, ev.NewMana.(*WeightedBaseMana).mana1.EffectiveBaseMana1, delta)
	assert.InDelta(t, 5, ev.NewMana.(*WeightedBaseMana).mana2.BaseMana2, delta)
	assert.InDelta(t, 3.465731, ev.NewMana.(*WeightedBaseMana).mana2.EffectiveBaseMana2, delta)
	assert.Equal(t, updateTime, ev.NewMana.LastUpdate())
}

func TestWeightedBaseManaVector_UpdateError(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
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
	bmv.SetMana(randID, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:   10,
			LastUpdated: baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:   10,
			LastUpdated: baseTime,
		},
		weight: Mixed,
	})
	// vector update to baseTime + 6 hours already
	err = bmv.Update(randID, baseTime)
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyUpdated, err)
}

func TestWeightedBaseManaVector_UpdateAll(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
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
	bmv.SetMana(inputPledgeID1, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:   10,
			LastUpdated: baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:   10,
			LastUpdated: baseTime,
		},
		weight: Mixed,
	})
	bmv.SetMana(inputPledgeID2, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:   10,
			LastUpdated: baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:   10,
			LastUpdated: baseTime,
		},
		weight: Mixed,
	})
	bmv.SetMana(inputPledgeID3, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:   10,
			LastUpdated: baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:   10,
			LastUpdated: baseTime,
		},
		weight: Mixed,
	})

	updateTime := baseTime.Add(time.Hour)
	err = bmv.UpdateAll(updateTime)
	assert.NoError(t, err)

	bmv.ForEach(func(id identity.ID, mana BaseMana) bool {
		assert.Equal(t, updateTime, mana.LastUpdate())
		return true
	})

	assert.Equal(t, 3, len(updateEvents))
	for _, ev := range updateEvents {
		assert.Contains(t, updatedNodeIds, ev.NodeID)
		delete(updatedNodeIds, ev.NodeID)
	}
	assert.Empty(t, updatedNodeIds)
}

func TestWeightedBaseManaVector_GetMana(t *testing.T) {
	t.Run("CASE: 0.5 weight", func(t *testing.T) {
		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
		assert.NoError(t, err)
		randID := randNodeID()
		mana, _, err := bmv.GetMana(randID)
		assert.Equal(t, 0.0, mana)
		assert.Error(t, err)

		now := time.Now()
		bmv.SetMana(randID, &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          10,
				EffectiveBaseMana1: 10,
				LastUpdated:        now,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          1,
				EffectiveBaseMana2: 1,
				LastUpdated:        now,
			},
			weight: Mixed,
		})

		mana, _, err = bmv.GetMana(randID)
		assert.NoError(t, err)
		assert.InDelta(t, 5.5, mana, delta)
	})

	t.Run("CASE: OnlyMana1 weight", func(t *testing.T) {
		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, OnlyMana1)
		assert.NoError(t, err)
		randID := randNodeID()
		mana, _, err := bmv.GetMana(randID)
		assert.Equal(t, 0.0, mana)
		assert.Error(t, err)

		now := time.Now()
		bmv.SetMana(randID, &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          10,
				EffectiveBaseMana1: 10,
				LastUpdated:        now,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          1,
				EffectiveBaseMana2: 1,
				LastUpdated:        now,
			},
			weight: OnlyMana1,
		})

		mana, _, err = bmv.GetMana(randID)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, mana, delta)
	})

	t.Run("CASE: OnlyMana2 weight", func(t *testing.T) {
		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, OnlyMana2)
		assert.NoError(t, err)
		randID := randNodeID()
		mana, _, err := bmv.GetMana(randID)
		assert.Equal(t, 0.0, mana)
		assert.Error(t, err)

		now := time.Now()
		bmv.SetMana(randID, &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          10,
				EffectiveBaseMana1: 10,
				LastUpdated:        now,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          1,
				EffectiveBaseMana2: 1,
				LastUpdated:        now,
			},
			weight: OnlyMana2,
		})

		mana, _, err = bmv.GetMana(randID)
		assert.NoError(t, err)
		assert.InDelta(t, 1.0, mana, delta)
	})
}

func TestWeightedBaseManaVector_ForEach(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)

	for i := 0; i < 10000; i++ {
		bmv.SetMana(randNodeID(), &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1: 1.5,
			},
			mana2: &AccessBaseMana{
				BaseMana2: 0.5,
			},
			weight: Mixed,
		})
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

func TestWeightedBaseManaVector_GetManaMap(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)

	// empty vector returns empty map
	manaMap, _, err := bmv.GetManaMap()
	assert.NoError(t, err)
	assert.Empty(t, manaMap)

	now := time.Now()
	nodeIDs := map[identity.ID]int{}

	for i := 0; i < 100; i++ {
		id := randNodeID()
		bmv.SetMana(id, &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          10,
				EffectiveBaseMana1: 10,
				LastUpdated:        now,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          1,
				EffectiveBaseMana2: 1,
				LastUpdated:        now,
			},
			weight: Mixed,
		})
		nodeIDs[id] = 0
	}

	manaMap, _, err = bmv.GetManaMap()
	assert.NoError(t, err)
	assert.Equal(t, 100, len(manaMap))
	for nodeID, mana := range manaMap {
		assert.InDelta(t, 5.5, mana, delta)
		assert.Contains(t, nodeIDs, nodeID)
		delete(nodeIDs, nodeID)
	}
	assert.Empty(t, nodeIDs)
}

func TestWeightedBaseManaVector_GetHighestManaNodes(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)

	nodeIDs := make([]identity.ID, 10)

	baseTime = time.Now()

	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          float64(i),
				EffectiveBaseMana1: float64(i),
				LastUpdated:        baseTime,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          float64(i),
				EffectiveBaseMana2: float64(i),
				LastUpdated:        baseTime,
			},
			weight: Mixed,
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
	for index, value := range result {
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}
}

func TestWeightedBaseManaVector_GetHighestManaNodesFraction(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)

	nodeIDs := make([]identity.ID, 10)

	baseTime = time.Now()

	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          float64(i),
				EffectiveBaseMana1: float64(i),
				LastUpdated:        baseTime,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          float64(i),
				EffectiveBaseMana2: float64(i),
				LastUpdated:        baseTime,
			},
			weight: Mixed,
		})
	}
	// requesting minus value
	result, _, err := bmv.GetHighestManaNodesFraction(-0.1)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	assert.Equal(t, nodeIDs[9], result[0].ID)
	assert.InDelta(t, 9.0, result[0].Mana, delta)

	// requesting the holders of top 10% of mana
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
	for index, value := range result {
		assert.Equal(t, nodeIDs[9-index], value.ID)
	}
}

func TestWeightedBaseManaVector_SetMana(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	nodeIDs := make([]identity.ID, 10)
	for i := 0; i < 10; i++ {
		nodeIDs[i] = randNodeID()
		bmv.SetMana(nodeIDs[i], &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          float64(i),
				EffectiveBaseMana1: float64(i),
				LastUpdated:        baseTime,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          float64(i),
				EffectiveBaseMana2: float64(i),
				LastUpdated:        baseTime,
			},
			weight: Mixed,
		})
	}
	for i := 0; i < 10; i++ {
		assert.Equal(t, WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          float64(i),
				EffectiveBaseMana1: float64(i),
				LastUpdated:        baseTime,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          float64(i),
				EffectiveBaseMana2: float64(i),
				LastUpdated:        baseTime,
			},
			weight: Mixed,
		}, *bmv.(*WeightedBaseManaVector).vector[nodeIDs[i]])
	}
}

func TestWeightedBaseManaVector_ToPersistables(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	id1 := randNodeID()
	id2 := randNodeID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          data[id1],
			EffectiveBaseMana1: data[id1],
			LastUpdated:        baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          data[id1],
			EffectiveBaseMana2: data[id1],
			LastUpdated:        baseTime,
		},
		weight: Mixed,
	})
	bmv.SetMana(id2, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          data[id2],
			EffectiveBaseMana1: data[id2],
			LastUpdated:        baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          data[id2],
			EffectiveBaseMana2: data[id2],
			LastUpdated:        baseTime,
		},
		weight: Mixed,
	})

	persistables := bmv.ToPersistables()

	assert.Equal(t, 2, len(persistables))
	for _, p := range persistables {
		assert.Equal(t, p.ManaType, WeightedMana)
		assert.Equal(t, p.LastUpdated, baseTime)
		assert.Equal(t, 2, len(p.BaseValues))
		assert.Equal(t, 2, len(p.EffectiveValues))
		assert.Equal(t, data[p.NodeID], p.BaseValues[0])
		assert.Equal(t, data[p.NodeID], p.BaseValues[1])
		assert.Equal(t, data[p.NodeID], p.EffectiveValues[0])
		assert.Equal(t, data[p.NodeID], p.EffectiveValues[1])
		delete(data, p.NodeID)
	}
	assert.Equal(t, 0, len(data))
}

func TestWeightedBaseManaVector_FromPersistable(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		id := randNodeID()
		p := &PersistableBaseMana{
			ManaType:        WeightedMana,
			BaseValues:      []float64{10, 50},
			EffectiveValues: []float64{100, 500},
			LastUpdated:     baseTime,
			NodeID:          id,
		}

		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
		assert.NoError(t, err)
		assert.False(t, bmv.Has(id))
		err = bmv.FromPersistable(p)
		assert.NoError(t, err)
		assert.True(t, bmv.Has(id))
		assert.Equal(t, 1, bmv.Size())
		bmValue := bmv.(*WeightedBaseManaVector).vector[id]
		assert.Equal(t, 30.0, bmValue.BaseValue())
		assert.Equal(t, 300.0, bmValue.EffectiveValue())
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

		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
		assert.NoError(t, err)

		err = bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has type Consensus instead of Weighted")
	})

	t.Run("CASE: Wrong number of base values", func(t *testing.T) {
		p := &PersistableBaseMana{
			ManaType:        WeightedMana,
			BaseValues:      []float64{0},
			EffectiveValues: []float64{0, 0},
			LastUpdated:     baseTime,
			NodeID:          randNodeID(),
		}

		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
		assert.NoError(t, err)
		err = bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has 1 base values instead of 2")
	})

	t.Run("CASE: Wrong number of effective values", func(t *testing.T) {
		p := &PersistableBaseMana{
			ManaType:        WeightedMana,
			BaseValues:      []float64{0, 0},
			EffectiveValues: []float64{0, 0, 0},
			LastUpdated:     baseTime,
			NodeID:          randNodeID(),
		}

		bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
		assert.NoError(t, err)

		err = bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has 3 effective values instead of 2")
	})
}

func TestWeightedBaseManaVector_ToAndFromPersistable(t *testing.T) {
	bmv, err := NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)
	id1 := randNodeID()
	id2 := randNodeID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          data[id1],
			EffectiveBaseMana1: data[id1],
			LastUpdated:        baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          data[id1],
			EffectiveBaseMana2: data[id1],
			LastUpdated:        baseTime,
		},
		weight: Mixed,
	})
	bmv.SetMana(id2, &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          data[id2],
			EffectiveBaseMana1: data[id2],
			LastUpdated:        baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          data[id2],
			EffectiveBaseMana2: data[id2],
			LastUpdated:        baseTime,
		},
		weight: Mixed,
	})

	persistables := bmv.ToPersistables()

	var restoredBmv BaseManaVector
	restoredBmv, err = NewResearchBaseManaVector(WeightedMana, AccessMana, Mixed)
	assert.NoError(t, err)

	for _, p := range persistables {
		err = restoredBmv.FromPersistable(p)
		assert.NoError(t, err)
	}
	assert.Equal(t, bmv.(*WeightedBaseManaVector).vector, restoredBmv.(*WeightedBaseManaVector).vector)
}
