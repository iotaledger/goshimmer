package sybilprotection

import (
	"context"

	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Weight struct {
	Value      int64       `serix:"0"`
	UpdateTime epoch.Index `serix:"1"`
}

func NewWeight(value int64, updateTime epoch.Index) (newWeight *Weight) {
	return &Weight{
		Value:      value,
		UpdateTime: updateTime,
	}
}

func (w Weight) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), w)
}

func (w *Weight) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, w)
}
