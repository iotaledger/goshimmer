package mana

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestRevoke_Consensus(t *testing.T) {
	bm := &ConsensusBaseMana{
		BaseMana1: 1.0,
	}
	err := bm.revoke(1.0)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.BaseMana1)
}

func TestRevokeNegativeBalance_Consensus(t *testing.T) {
	bm := &ConsensusBaseMana{
		BaseMana1: 0.0,
	}
	err := bm.revoke(1.0)
	assert.NoError(t, err)
	assert.EqualValues(t, -1, bm.BaseMana1)
}

func TestPledgeAndUpdateRegularOldFunds_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := &ConsensusBaseMana{
		BaseMana1: 1.0,
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
