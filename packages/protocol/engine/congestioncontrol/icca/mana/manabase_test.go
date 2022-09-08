package mana

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
)

func TestRevoke_Consensus(t *testing.T) {
	bm := NewManaBase(1.0)
	err := bm.revoke(1.0)
	assert.NoError(t, err)
	// values are only valid for default coefficients of 0.00003209 and t = 6 hours
	assert.Equal(t, 0.0, bm.BaseValue())
}

func TestRevokeNegativeBalance_Consensus(t *testing.T) {
	bm := NewManaBase(0.0)

	err := bm.revoke(1.0)
	assert.NoError(t, err)
	assert.EqualValues(t, -1, bm.BaseValue())
}

func TestPledgeAndUpdateRegularOldFunds_Consensus(t *testing.T) {
	_baseTime := time.Now()
	bm := NewManaBase(1.0)

	// transaction pledges mana at t=6 hours with 3 inputs.
	_txInfo := &TxInfo{
		TimeStamp:    _baseTime.Add(time.Hour * 6),
		TotalBalance: 10.0,
		PledgeID:     map[Type]identity.ID{}, // don't care
		InputInfos: []InputInfo{
			{
				Amount:   5.0,
				PledgeID: map[Type]identity.ID{}, // don't care
			},
			{
				Amount:   3.0,
				PledgeID: map[Type]identity.ID{}, // don't care
			},
			{
				Amount:   2.0,
				PledgeID: map[Type]identity.ID{}, // don't care
			},
		},
	}

	bm.pledge(_txInfo.sumInputs())

	assert.Equal(t, 10.0, _txInfo.sumInputs())
	assert.Equal(t, 11.0, bm.BaseValue())
}
