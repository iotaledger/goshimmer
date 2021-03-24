package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"golang.org/x/xerrors"
)

// delta interval for checking quasi-equivalence of float64 values
var delta = 0.0001

func TestUpdateEBM1_Weighted(t *testing.T) {
	bm1 := NewWeightedMana(Mixed)
	bm2 := NewWeightedMana(Mixed)

	// 0 initial values, timely update should not change anything
	bm1.mana1.updateEBM1(time.Second)
	assert.Equal(t, 0.0, bm1.mana1.EffectiveBaseMana1)

	// Only update once
	// BM1 was pledged a t = 0
	bm1.mana1.BaseMana1 = 1.0
	// with default values of 0.00003209
	bm1.mana1.updateEBM1(time.Hour * 6)
	assert.InDelta(t, 0.5, bm1.mana1.EffectiveBaseMana1, delta)

	// Update regularly
	// BM1 was pledged a t = 0
	bm2.mana1.BaseMana1 = 1.0
	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
	for i := 0; i < 6; i++ {
		bm2.mana1.updateEBM1(time.Hour)
	}
	assert.InDelta(t, 0.5, bm2.mana1.EffectiveBaseMana1, delta)
}

func TestUpdateBM2_Weighted(t *testing.T) {
	bm1 := NewWeightedMana(Mixed)
	bm2 := NewWeightedMana(Mixed)

	// 0 initial values, timely update should not change anything
	bm1.mana2.updateBM2(time.Hour)
	assert.Equal(t, 0.0, bm1.mana2.BaseMana2)

	// pledge BM2 at t = o
	bm1.mana2.BaseMana2 = 1.0
	bm1.mana2.updateBM2(time.Hour * 6)
	assert.InDelta(t, 0.5, bm1.mana2.BaseMana2, delta)

	// pledge BM2 at t = o
	bm2.mana2.BaseMana2 = 1.0
	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
	for i := 0; i < 6; i++ {
		bm2.mana2.updateBM2(time.Hour)
	}
	assert.InDelta(t, 0.5, bm2.mana2.BaseMana2, delta)
}

func TestUpdateEBM2CoeffEqual_Weighted(t *testing.T) {
	bm1 := NewWeightedMana(Mixed)
	bm2 := NewWeightedMana(Mixed)

	// 0 initial values, timely update should not change anything
	bm1.mana2.updateEBM2(time.Hour)
	assert.Equal(t, 0.0, bm1.mana2.EffectiveBaseMana2)

	// first, let's calculate once on a 6 hour span
	// pledge BM2 at t = o
	bm1.mana2.BaseMana2 = 1.0
	// updateEBM2 relies on an update baseMana2 value
	bm1.mana2.updateBM2(time.Hour * 6)
	bm1.mana2.updateEBM2(time.Hour * 6)

	// second, let's calculate the same but every hour
	// pledge BM2 at t = o
	bm2.mana2.BaseMana2 = 1.0
	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
	for i := 0; i < 6; i++ {
		// updateEBM2 relies on an update baseMana2 value
		bm2.mana2.updateBM2(time.Hour)
		bm2.mana2.updateEBM2(time.Hour)
	}

	// compare results of the two calculations
	assert.Equal(t, true, math.Abs(bm1.mana2.EffectiveBaseMana2-bm2.mana2.EffectiveBaseMana2) < delta)
}

//func TestUpdateEBM2CoeffNotEqual(t *testing.T) {
//	// set coefficients such that emaCoeff2 != Decay
//	SetCoefficients(emaCoeff1, 0.0004, Decay)
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
//	assert.Equal(t, true, math.Abs(bm1.EffectiveBaseMana2-bm2.EffectiveBaseMana2) < delta)
//}

func TestUpdateTimeInPast_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          1.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}

	pastTime := _baseTime.Add(time.Hour * -1)
	err := bm.update(pastTime)
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyUpdated, err)
}

func TestUpdate_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          1.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}
	updateTime := _baseTime.Add(time.Hour * 6)

	err := bm.update(updateTime)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 1.0, bm.mana1.BaseMana1)
	assert.InDelta(t, 0.5, bm.mana1.EffectiveBaseMana1, delta)
	assert.InDelta(t, 0.5, bm.mana2.BaseMana2, delta)
	assert.InDelta(t, 0.346573, bm.mana2.EffectiveBaseMana2, delta)
	assert.Equal(t, updateTime, bm.mana1.LastUpdated)
}

func TestRevokeRegular_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          1.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}
	revokeTime := _baseTime.Add(time.Hour * 6)
	err := bm.revoke(1.0, revokeTime)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.mana1.BaseMana1)
	assert.InDelta(t, 0.5, bm.mana1.EffectiveBaseMana1, delta)
	assert.InDelta(t, 0.5, bm.mana2.BaseMana2, delta)
	assert.InDelta(t, 0.346573, bm.mana2.EffectiveBaseMana2, delta)
	assert.Equal(t, revokeTime, bm.mana1.LastUpdated)
}

func TestRevokeRegularNegativeBalance_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          0.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}
	revokeTime := _baseTime.Add(time.Hour * 6)
	err := bm.revoke(1.0, revokeTime)
	assert.Error(t, err)
	assert.Equal(t, ErrBaseManaNegative, err)
}

func TestRevokePast_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          1.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}
	// update values until t = 6 hours
	updateTime := _baseTime.Add(time.Hour * 6)
	err := bm.update(updateTime)
	assert.NoError(t, err)
	assert.InDelta(t, 0.5, bm.mana1.EffectiveBaseMana1, delta)

	// revoke base mana 1 at t=0 in the past
	err = bm.revoke(1.0, _baseTime)
	assert.NoError(t, err)

	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.mana1.BaseMana1)
	assert.InDelta(t, 0.0, bm.mana1.EffectiveBaseMana1, delta)
	assert.InDelta(t, 0.5, bm.mana2.BaseMana2, delta)
	assert.InDelta(t, 0.346573, bm.mana2.EffectiveBaseMana2, delta)
	assert.Equal(t, updateTime, bm.mana1.LastUpdated)
}

func TestRevokePastNegativeBalance_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          0.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}
	// update values until t = 6 hours
	updateTime := _baseTime.Add(time.Hour * 6)
	err := bm.update(updateTime)
	assert.NoError(t, err)
	assert.InDelta(t, 0.0, bm.mana1.EffectiveBaseMana1, delta)

	err = bm.revoke(1.0, _baseTime)
	assert.Error(t, err)
	assert.Equal(t, ErrBaseManaNegative, err)
}

func TestPledgeRegularOldFunds_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          1.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}

	// transaction pledges mana at t=6 hours with 3 inputs.
	_txInfo := &TxInfo{
		TimeStamp:    _baseTime.Add(time.Hour * 6),
		TotalBalance: 10.0,
		PledgeID:     map[Type]identity.ID{}, // don't care
		InputInfos: []InputInfo{
			{
				// funds have been sitting here for couple days...
				TimeStamp: _baseTime.Add(time.Hour * -200),
				Amount:    5.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: _baseTime.Add(time.Hour * -200),
				Amount:    3.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: _baseTime.Add(time.Hour * -200),
				Amount:    2.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
		},
	}

	pledged := bm.pledge(_txInfo)

	assert.InDelta(t, 10.0, pledged, delta)
	assert.Equal(t, 11.0, bm.mana1.BaseMana1)
	// half of the original BM2 degraded away in 6 hours
	assert.InDelta(t, 10.5, bm.mana2.BaseMana2, delta)
}

func TestPledgeRegularHalfOldFunds_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          1.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          1.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime,
		},
		weight: Mixed,
	}

	// transaction pledges mana at t=6 hours with 3 inputs.
	_txInfo := &TxInfo{
		TimeStamp:    _baseTime.Add(time.Hour * 6),
		TotalBalance: 10.0,
		PledgeID:     map[Type]identity.ID{}, // don't care
		InputInfos: []InputInfo{
			{
				// funds have been sitting here for 6 hours
				TimeStamp: _baseTime,
				Amount:    5.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for 6 hours
				TimeStamp: _baseTime,
				Amount:    3.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for 6 hours
				TimeStamp: _baseTime,
				Amount:    2.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
		},
	}

	pledged := bm.pledge(_txInfo)

	assert.InDelta(t, 7.5, pledged, delta)
	assert.Equal(t, 11.0, bm.mana1.BaseMana1)
	// half of the original BM2 degraded away in 6 hours
	assert.InDelta(t, 5.5, bm.mana2.BaseMana2, delta)
}

func TestPledgPastOldFunds_Weighted(t *testing.T) {
	_baseTime := time.Now()
	bm := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          0.0,
			EffectiveBaseMana1: 0.0,
			LastUpdated:        _baseTime.Add(time.Hour * 6),
		},
		mana2: &AccessBaseMana{
			BaseMana2:          0.0,
			EffectiveBaseMana2: 0.0,
			LastUpdated:        _baseTime.Add(time.Hour * 6),
		},
		weight: Mixed,
	}

	// transaction pledges mana at t=0 hours with 3 inputs.
	_txInfo := &TxInfo{
		TimeStamp:    _baseTime,
		TotalBalance: 10.0,
		PledgeID:     map[Type]identity.ID{}, // don't care
		InputInfos: []InputInfo{
			{
				// funds have been sitting here for couple days...
				TimeStamp: _baseTime.Add(time.Hour * -200),
				Amount:    5.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: _baseTime.Add(time.Hour * -200),
				Amount:    3.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
			{
				// funds have been sitting here for couple days...
				TimeStamp: _baseTime.Add(time.Hour * -200),
				Amount:    2.0,
				PledgeID:  map[Type]identity.ID{}, // don't care
			},
		},
	}

	pledged := bm.pledge(_txInfo)

	// pledged at t=0, half of input amount is added to bm2
	assert.InDelta(t, 7.5, pledged, delta)
	assert.Equal(t, 10.0, bm.mana1.BaseMana1)
	// half of the original BM2 degraded away in 6 hours
	assert.InDelta(t, 5.0, bm.mana2.BaseMana2, delta)
	// valid EBM2 at t=6 hours, after pledging 10 BM2 at t=0
	assert.InDelta(t, 3.465731, bm.mana2.EffectiveBaseMana2, delta)
}

func TestWeightedBaseMana_SetWeight(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		bm := &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          1.0,
				EffectiveBaseMana1: 0.0,
				LastUpdated:        baseTime,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          1.0,
				EffectiveBaseMana2: 0.0,
				LastUpdated:        baseTime,
			},
			weight: Mixed,
		}
		err := bm.SetWeight(OnlyMana1)
		assert.NoError(t, err)
		assert.Equal(t, OnlyMana1, bm.weight)
	})
	t.Run("CASE: Negative weight", func(t *testing.T) {
		bm := &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          1.0,
				EffectiveBaseMana1: 0.0,
				LastUpdated:        baseTime,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          1.0,
				EffectiveBaseMana2: 0.0,
				LastUpdated:        baseTime,
			},
			weight: Mixed,
		}
		err := bm.SetWeight(-0.5)
		assert.Error(t, err)
		assert.True(t, xerrors.Is(err, ErrInvalidWeightParameter))
	})

	t.Run("CASE: Too big", func(t *testing.T) {
		bm := &WeightedBaseMana{
			mana1: &ConsensusBaseMana{
				BaseMana1:          1.0,
				EffectiveBaseMana1: 0.0,
				LastUpdated:        baseTime,
			},
			mana2: &AccessBaseMana{
				BaseMana2:          1.0,
				EffectiveBaseMana2: 0.0,
				LastUpdated:        baseTime,
			},
			weight: Mixed,
		}
		err := bm.SetWeight(1.1)
		assert.Error(t, err)
		assert.True(t, xerrors.Is(err, ErrInvalidWeightParameter))
	})
}
