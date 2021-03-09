package mana

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestUpdateEBM1(t *testing.T) {
	bm1 := ConsensusBaseMana{}
	bm2 := ConsensusBaseMana{}

	// 0 initial values, timely update should not change anything
	bm1.updateEBM1(time.Second)
	assert.Equal(t, 0.0, bm1.EffectiveBaseMana1)

	// Only update once
	// BM1 was pledged a t = 0
	bm1.BaseMana1 = 1.0
	// with default values of 0.00003209
	bm1.updateEBM1(time.Hour * 6)
	assert.InDelta(t, 0.5, bm1.EffectiveBaseMana1, delta)

	// Update regularly
	// BM1 was pledged a t = 0
	bm2.BaseMana1 = 1.0
	// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
	for i := 0; i < 6; i++ {
		bm2.updateEBM1(time.Hour)
	}
	assert.InDelta(t, 0.5, bm2.EffectiveBaseMana1, delta)
}

func TestUpdateTimeInPast_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
	}
	pastTime := _baseTime.Add(time.Hour * -1)
	err := bm.update(pastTime)
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyUpdated, err)
}

func TestUpdate_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
	}
	updateTime := _baseTime.Add(time.Hour * 6)

	err := bm.update(updateTime)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 1.0, bm.BaseMana1)
	assert.InDelta(t, 0.5, bm.EffectiveBaseMana1, delta)
	assert.Equal(t, updateTime, bm.LastUpdated)
}

func TestRevoke_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
	}
	revokeTime := _baseTime.Add(time.Hour * 6)
	err := bm.revoke(1.0, revokeTime)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.BaseMana1)
	assert.InDelta(t, 0.5, bm.EffectiveBaseMana1, delta)
	assert.Equal(t, revokeTime, bm.LastUpdated)
}

func TestRevokeNegativeBalance_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          0.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
	}
	revokeTime := _baseTime.Add(time.Hour * 6)
	err := bm.revoke(1.0, revokeTime)
	assert.Error(t, err)
	assert.Equal(t, ErrBaseManaNegative, err)
}

func TestRevokePast_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
	}
	// update values until t = 6 hours
	updateTime := _baseTime.Add(time.Hour * 6)
	err := bm.update(updateTime)
	assert.NoError(t, err)
	assert.InDelta(t, 0.5, bm.EffectiveBaseMana1, delta)

	// revoke base mana 1 at t=0 in the past
	err = bm.revoke(1.0, _baseTime)
	assert.NoError(t, err)

	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.BaseMana1)
	assert.InDelta(t, 0.0, bm.EffectiveBaseMana1, delta)
	assert.Equal(t, updateTime, bm.LastUpdated)
}

func TestRevokePastNegativeBalance_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          0.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
	}
	// update values until t = 6 hours
	updateTime := _baseTime.Add(time.Hour * 6)
	err := bm.update(updateTime)
	assert.NoError(t, err)
	assert.InDelta(t, 0.0, bm.EffectiveBaseMana1, delta)

	err = bm.revoke(1.0, _baseTime)
	assert.Error(t, err)
	assert.Equal(t, ErrBaseManaNegative, err)
}

func TestPledgeAndUpdateRegularOldFunds_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
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

	assert.Equal(t, 10.0, pledged)
	assert.Equal(t, 11.0, bm.BaseMana1)
}

func TestPledgeRegularHalfOldFunds_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          1.0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime,
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

	bm1Pledged := bm.pledge(_txInfo)

	assert.Equal(t, 10.0, bm1Pledged)
	assert.Equal(t, 11.0, bm.BaseMana1)
}

func TestPledgePastOldFunds_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1:          0,
		EffectiveBaseMana1: 0.0,
		LastUpdated:        _baseTime.Add(time.Hour * 6),
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

	bm1Pledged := bm.pledge(_txInfo)

	assert.Equal(t, 10.0, bm1Pledged)
	assert.Equal(t, 10.0, bm.BaseMana1)
}
