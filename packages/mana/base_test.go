package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"

	"github.com/stretchr/testify/assert"
)

// epsilon interval for checking quasi-equivalence of float64 values
var eps = 0.0001

func within(target float64, actual float64) bool {
	return math.Abs(target-actual) < eps
}

func TestUpdateEBM1(t *testing.T) {
	bm1 := BaseMana{}
	bm2 := BaseMana{}

	// 0 initial values, timely update should not change anything
	bm1.updateEBM1(time.Second)
	assert.Equal(t, 0.0, bm1.EffectiveBaseMana1)

	// Only update once
	// BM1 was pledged a t = 0
	bm1.BaseMana1 = 1.0
	// with default values of 0.00003209
	bm1.updateEBM1(time.Hour * 6)
	assert.Equal(t, true, within(0.5, bm1.EffectiveBaseMana1))

	// Update regularly
	// BM1 was pledged a t = 0
	bm2.BaseMana1 = 1.0
	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
	for i := 0; i < 6; i++ {
		bm2.updateEBM1(time.Hour)
	}
	assert.Equal(t, true, within(0.5, bm2.EffectiveBaseMana1))
}

func TestUpdateBM2(t *testing.T) {
	bm1 := BaseMana{}
	bm2 := BaseMana{}

	// 0 initial values, timely update should not change anything
	bm1.updateBM2(time.Hour)
	assert.Equal(t, 0.0, bm1.BaseMana2)

	// pledge BM2 at t = o
	bm1.BaseMana2 = 1.0
	bm1.updateBM2(time.Hour * 6)
	assert.Equal(t, true, within(0.5, bm1.BaseMana2))

	// pledge BM2 at t = o
	bm2.BaseMana2 = 1.0
	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
	for i := 0; i < 6; i++ {
		bm2.updateBM2(time.Hour)
	}
	assert.Equal(t, true, within(0.5, bm2.BaseMana2))
}

func TestUpdateEBM2CoeffEqual(t *testing.T) {
	bm1 := BaseMana{}
	bm2 := BaseMana{}

	// 0 initial values, timely update should not change anything
	bm1.updateEBM2(time.Hour)
	assert.Equal(t, 0.0, bm1.EffectiveBaseMana2)

	// first, let's calculate once on a 6 hour span
	// pledge BM2 at t = o
	bm1.BaseMana2 = 1.0
	// updateEBM2 relies on an update baseMana2 value
	bm1.updateBM2(time.Hour * 6)
	bm1.updateEBM2(time.Hour * 6)

	// second, let's calculate the same but every hour
	// pledge BM2 at t = o
	bm2.BaseMana2 = 1.0
	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
	for i := 0; i < 6; i++ {
		// updateEBM2 relies on an update baseMana2 value
		bm2.updateBM2(time.Hour)
		bm2.updateEBM2(time.Hour)
	}

	// compare results of the two calculations
	assert.Equal(t, true, math.Abs(bm1.EffectiveBaseMana2-bm2.EffectiveBaseMana2) < eps)
}

//func TestUpdateEBM2CoeffNotEqual(t *testing.T) {
//	// set coefficients such that emaCoeff2 != decay
//	SetCoefficients(emaCoeff1, 0.0004, decay)
//	bm1 := BaseMana{}
//	bm2 := BaseMana{}
//
//	// 0 initial values, timely update should not change anything
//	bm1.updateEBM2(time.Hour)
//	assert.Equal(t, 0.0, bm1.EffectiveBaseMana2)
//
//	// first, let's calculate once on a 6 hour span
//	// pledge BM2 at t = o
//	bm1.BaseMana2 = 1.0
//	// updateEBM2 relies on an update baseMana2 value
//	bm1.updateBM2(time.Hour * 6)
//	bm1.updateEBM2(time.Hour * 6)
//
//	// second, let's calculate the same but every hour
//	// pledge BM2 at t = o
//	bm2.BaseMana2 = 1.0
//	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
//	for i := 0; i < 6; i++ {
//		// updateEBM2 relies on an update baseMana2 value
//		bm2.updateBM2(time.Hour)
//		bm2.updateEBM2(time.Hour)
//	}
//
//	// compare results of the two calculations
//	assert.Equal(t, true, math.Abs(bm1.EffectiveBaseMana2-bm2.EffectiveBaseMana2) < eps)
//}

func TestUpdateTimeInPast(t *testing.T) {
	baseTime := time.Now()
	bm := &BaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}
	pastTime := baseTime.Add(time.Hour * -1)
	err := bm.update(pastTime)
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyUpdated, err)
}

func TestUpdate(t *testing.T) {
	baseTime := time.Now()
	bm := &BaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}
	updateTime := baseTime.Add(time.Hour * 6)

	err := bm.update(updateTime)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 1.0, bm.BaseMana1)
	assert.Equal(t, true, within(0.5, bm.EffectiveBaseMana1))
	assert.Equal(t, true, within(0.5, bm.BaseMana2))
	assert.Equal(t, true, within(0.346573, bm.EffectiveBaseMana2))
	assert.Equal(t, updateTime, bm.LastUpdated)
}

func TestRevokeBaseMana1Regular(t *testing.T) {
	baseTime := time.Now()
	bm := &BaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}
	revokeTime := baseTime.Add(time.Hour * 6)
	bm.revokeBaseMana1(1.0, revokeTime)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.BaseMana1)
	assert.Equal(t, true, within(0.5, bm.EffectiveBaseMana1))
	assert.Equal(t, true, within(0.5, bm.BaseMana2))
	assert.Equal(t, true, within(0.346573, bm.EffectiveBaseMana2))
	assert.Equal(t, revokeTime, bm.LastUpdated)
}

func TestRevokeBaseMana1Past(t *testing.T) {
	baseTime := time.Now()
	bm := &BaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}
	// update values until t = 6 hours
	updateTime := baseTime.Add(time.Hour * 6)
	bm.update(updateTime)

	assert.Equal(t, true, within(0.5, bm.EffectiveBaseMana1))

	// revoke base mana 1 at t=0 in the past
	bm.revokeBaseMana1(1.0, baseTime)

	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.BaseMana1)
	assert.Equal(t, true, within(0.0, bm.EffectiveBaseMana1))
	assert.Equal(t, true, within(0.5, bm.BaseMana2))
	assert.Equal(t, true, within(0.346573, bm.EffectiveBaseMana2))
	assert.Equal(t, updateTime, bm.LastUpdated)
}

func TestPledgeAndUpdateRegularOldFunds(t *testing.T) {
	baseTime := time.Now()
	bm := &BaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}

	// transaction pledges mana at t=6 hours with 3 inputs.
	txInfo := &TxInfo{
		TimeStamp:    baseTime.Add(time.Hour * 6),
		TotalBalance: 10.0,
		PledgeID:     map[Type]identity.ID{}, // don't care
		InputInfos: []InputInfo{
			{
				// funds have been sitting here for couple days...
				TimeStamp: baseTime.Add(time.Hour * -200),
				Amount:    5.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: baseTime.Add(time.Hour * -200),
				Amount:    3.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: baseTime.Add(time.Hour * -200),
				Amount:    2.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
		},
	}

	bm1Pledged, bm2Pledged := bm.pledgeAndUpdate(txInfo)

	assert.Equal(t, 10.0, bm1Pledged)
	assert.True(t, within(10.0, bm2Pledged))
	assert.Equal(t, 11.0, bm.BaseMana1)
	// half of the original BM2 degraded away in 6 hours
	assert.True(t, within(10.5, bm.BaseMana2))
}

func TestPledgeAndUpdateRegularHalfOldFunds(t *testing.T) {
	baseTime := time.Now()
	bm := &BaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}

	// transaction pledges mana at t=6 hours with 3 inputs.
	txInfo := &TxInfo{
		TimeStamp:    baseTime.Add(time.Hour * 6),
		TotalBalance: 10.0,
		PledgeID:     map[Type]identity.ID{}, // don't care
		InputInfos: []InputInfo{
			{
				// funds have been sitting here for 6 hours
				TimeStamp: baseTime,
				Amount:    5.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for 6 hours
				TimeStamp: baseTime,
				Amount:    3.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for 6 hours
				TimeStamp: baseTime,
				Amount:    2.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
		},
	}

	bm1Pledged, bm2Pledged := bm.pledgeAndUpdate(txInfo)

	assert.Equal(t, 10.0, bm1Pledged)
	assert.True(t, within(5.0, bm2Pledged))
	assert.Equal(t, 11.0, bm.BaseMana1)
	// half of the original BM2 degraded away in 6 hours
	assert.True(t, within(5.5, bm.BaseMana2))
}

func TestPledgeAndUpdatePastOldFunds(t *testing.T) {
	baseTime := time.Now()
	bm := &BaseMana{
		BaseMana1:          0,
		EffectiveBaseMana1: 0.0,
		BaseMana2:          0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime.Add(time.Hour * 6),
	}

	// transaction pledges mana at t=0 hours with 3 inputs.
	txInfo := &TxInfo{
		TimeStamp:    baseTime,
		TotalBalance: 10.0,
		PledgeID:     map[Type]identity.ID{}, // don't care
		InputInfos: []InputInfo{
			{
				// funds have been sitting here for couple days...
				TimeStamp: baseTime.Add(time.Hour * -200),
				Amount:    5.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: baseTime.Add(time.Hour * -200),
				Amount:    3.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: baseTime.Add(time.Hour * -200),
				Amount:    2.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
		},
	}

	bm1Pledged, bm2Pledged := bm.pledgeAndUpdate(txInfo)

	assert.Equal(t, 10.0, bm1Pledged)
	// pledged at t=0, half of input amount is added to bm2
	assert.True(t, within(5.0, bm2Pledged))
	assert.Equal(t, 10.0, bm.BaseMana1)
	// half of the original BM2 degraded away in 6 hours
	assert.True(t, within(5.0, bm.BaseMana2))
	// valid EBM2 at t=6 hours, after pledging 10 BM2 at t=0
	assert.True(t, within(3.465731, bm.EffectiveBaseMana2))
}

func TestNodeMap_GetPercentile(t *testing.T) {
	nodes := make(NodeMap)
	nodes[identity.GenerateIdentity().ID()] = 1
	nodes[identity.GenerateIdentity().ID()] = 2
	nodes[identity.GenerateIdentity().ID()] = 3
	checkID := identity.GenerateIdentity().ID()
	nodes[checkID] = 4
	percentile, err := nodes.GetPercentile(checkID)
	assert.NoError(t, err)
	assert.Equal(t, 75.0, percentile)
}
