package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

var delta = 0.001

func TestUpdateBM2(t *testing.T) {
	t.Run("CASE: Zero values", func(t *testing.T) {
		bm := AccessBaseMana{}

		// 0 initial values, timely update should not change anything
		bm.updateBM2(time.Hour)
		assert.Equal(t, 0.0, bm.BaseMana2)
	})

	t.Run("CASE: Batch update", func(t *testing.T) {
		bm := AccessBaseMana{}

		// pledge BM2 at t = o
		bm.BaseMana2 = 1.0
		bm.updateBM2(time.Hour * 6)
		assert.InDelta(t, 0.5, bm.BaseMana2, delta)
	})

	t.Run("CASE: Incremental update", func(t *testing.T) {
		bm := AccessBaseMana{}

		// pledge BM2 at t = o
		bm.BaseMana2 = 1.0
		// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
		for i := 0; i < 6; i++ {
			bm.updateBM2(time.Hour)
		}
		assert.InDelta(t, 0.5, bm.BaseMana2, delta)
	})
}

func TestUpdateEBM2CoeffEqual(t *testing.T) {
	t.Run("CASE: Zero values", func(t *testing.T) {
		bm := AccessBaseMana{}

		// 0 initial values, timely update should not change anything
		bm.updateEBM2(time.Hour)
		assert.Equal(t, 0.0, bm.EffectiveBaseMana2)
	})

	t.Run("CASE: Batch and incremental update", func(t *testing.T) {
		bmBatch := AccessBaseMana{}

		// first, let's calculate once on a 6 hour span
		// pledge BM2 at t = o
		bmBatch.BaseMana2 = 1.0
		// updateEBM2 relies on an update baseMana2 value
		bmBatch.updateBM2(time.Hour * 6)
		bmBatch.updateEBM2(time.Hour * 6)

		bmInc := AccessBaseMana{}
		// second, let's calculate the same but every hour
		// pledge BM2 at t = o
		bmInc.BaseMana2 = 1.0
		// with emaCoeff1 = 0.00003209, half value should be reached within 6 hours
		for i := 0; i < 6; i++ {
			// updateEBM2 relies on an update baseMana2 value
			bmInc.updateBM2(time.Hour)
			bmInc.updateEBM2(time.Hour)
		}

		// compare results of the two calculations
		assert.Equal(t, true, math.Abs(bmBatch.EffectiveBaseMana2-bmInc.EffectiveBaseMana2) < delta)
	})

	t.Run("CASE: Large durations BM2", func(t *testing.T) {
		bmBatch := AccessBaseMana{}

		// first, let's calculate once on a 6 hour span
		// pledge BM2 at t = o
		bmBatch.BaseMana2 = 1.0
		// updateEBM2 relies on an update baseMana2 value
		minTime := time.Unix(-2208988800, 0) // Jan 1, 1900
		maxTime := minTime.Add(1<<63 - 1)

		bmBatch.updateBM2(maxTime.Sub(minTime))

		assert.False(t, math.IsNaN(bmBatch.BaseMana2))
		assert.False(t, math.IsInf(bmBatch.BaseMana2, 0))
		assert.Equal(t, 0.0, bmBatch.BaseMana2)
	})

	t.Run("CASE: Large durations EBM2 Decay==emaCoeff2", func(t *testing.T) {
		bmBatch := AccessBaseMana{}
		// pledge BM2 at t = o
		bmBatch.BaseMana2 = 1.0
		bmBatch.EffectiveBaseMana2 = 1.0

		// updateEBM2 relies on an update baseMana2 value
		minTime := time.Unix(-2208988800, 0) // Jan 1, 1900
		maxTime := minTime.Add(1<<63 - 1)

		bmBatch.updateBM2(maxTime.Sub(minTime))
		bmBatch.updateEBM2(maxTime.Sub(minTime))
		assert.False(t, math.IsNaN(bmBatch.BaseMana2))
		assert.False(t, math.IsNaN(bmBatch.EffectiveBaseMana2))
		assert.False(t, math.IsInf(bmBatch.BaseMana2, 0))
		assert.False(t, math.IsInf(bmBatch.EffectiveBaseMana2, 0))
		assert.Equal(t, 0.0, bmBatch.BaseMana2)
		assert.Equal(t, 0.0, bmBatch.EffectiveBaseMana2)
	})
	t.Run("CASE: Large durations EBM2 Decay!=emaCoeff2", func(t *testing.T) {
		bmBatch := AccessBaseMana{}
		SetCoefficients(0.00003209, 0.0057762265, 0.00003209)
		// pledge BM2 at t = o
		bmBatch.BaseMana2 = 1.0
		bmBatch.EffectiveBaseMana2 = 1.0

		// updateEBM2 relies on an update baseMana2 value
		minTime := time.Unix(-2208988800, 0) // Jan 1, 1900
		maxTime := minTime.Add(1<<63 - 1)

		bmBatch.updateBM2(maxTime.Sub(minTime))
		bmBatch.updateEBM2(maxTime.Sub(minTime))

		assert.False(t, math.IsNaN(bmBatch.BaseMana2))
		assert.False(t, math.IsNaN(bmBatch.EffectiveBaseMana2))
		assert.False(t, math.IsInf(bmBatch.BaseMana2, 0))
		assert.False(t, math.IsInf(bmBatch.EffectiveBaseMana2, 0))
		assert.Equal(t, 0.0, bmBatch.BaseMana2)
		assert.Equal(t, 0.0, bmBatch.EffectiveBaseMana2)
		// re-set the default values so that other tests pass
		SetCoefficients(0.00003209, 0.00003209, 0.00003209)
	})
}

func TestUpdateTimeInPast_Access(t *testing.T) {
	baseTime := time.Now()
	bm := &AccessBaseMana{
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}
	pastTime := baseTime.Add(time.Hour * -1)
	err := bm.update(pastTime)
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyUpdated, err)
}

func TestUpdate_Access(t *testing.T) {
	baseTime := time.Now()
	bm := &AccessBaseMana{
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}
	updateTime := baseTime.Add(time.Hour * 6)

	err := bm.update(updateTime)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.InDelta(t, 0.5, bm.BaseMana2, delta)
	assert.InDelta(t, 0.346573, bm.EffectiveBaseMana2, delta)
	assert.Equal(t, updateTime, bm.LastUpdated)
}

func TestRevoke_Access(t *testing.T) {
	baseTime := time.Now()
	bm := &AccessBaseMana{
		BaseMana2:          1.0,
		EffectiveBaseMana2: 0.0,
		LastUpdated:        baseTime,
	}
	assert.Panics(t, func() {
		_ = bm.revoke(1.0)
	})
}

func TestPledgeRegularOldFunds_Access(t *testing.T) {
	baseTime := time.Now()
	bm := &AccessBaseMana{
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

	bm2Pledged := bm.pledge(txInfo)

	assert.InDelta(t, 10.0, bm2Pledged, delta)
	// half of the original BM2 degraded away in 6 hours
	assert.InDelta(t, 10.5, bm.BaseMana2, delta)
}

func TestPledgeRegularHalfOldFunds_Access(t *testing.T) {
	baseTime := time.Now()
	bm := &AccessBaseMana{
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

	bm2Pledged := bm.pledge(txInfo)

	assert.InDelta(t, 5.0, bm2Pledged, delta)
	// half of the original BM2 degraded away in 6 hours
	assert.InDelta(t, 5.5, bm.BaseMana2, delta)
}

func TestPledgePastOldFunds_Access(t *testing.T) {
	baseTime := time.Now()
	bm := &AccessBaseMana{
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

	bm2Pledged := bm.pledge(txInfo)

	// pledged at t=0, half of input amount is added to bm2
	assert.InDelta(t, 5.0, bm2Pledged, delta)
	// half of the original BM2 degraded away in 6 hours
	// valid EBM2 at t=6 hours, after pledging 10 BM2 at t=0
	assert.InDelta(t, 3.465731, bm.EffectiveBaseMana2, delta)
}
