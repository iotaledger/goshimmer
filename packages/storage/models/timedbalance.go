package models

import (
	"context"

	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type TimedBalance struct {
	Balance     int64       `serix:"0"`
	LastUpdated epoch.Index `serix:"1"`
}

func NewTimedBalance(balance int64, lastUpdated epoch.Index) (timedBalance *TimedBalance) {
	return &TimedBalance{
		Balance:     balance,
		LastUpdated: lastUpdated,
	}
}

func (t TimedBalance) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), t)
}

func (t *TimedBalance) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, t)
}
