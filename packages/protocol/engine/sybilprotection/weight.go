package sybilprotection

import (
	"context"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

// Weight is a weight annotated with the slot it was last updated in.
type Weight struct {
	Value      int64      `serix:"0"`
	UpdateTime slot.Index `serix:"1"`
}

// NewWeight creates a new Weight instance.
func NewWeight(value int64, updateTime slot.Index) (newWeight *Weight) {
	return &Weight{
		Value:      value,
		UpdateTime: updateTime,
	}
}

// Bytes returns a serialized version of the Weight.
func (w Weight) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), w)
}

// FromBytes parses a serialized version of the Weight.
func (w *Weight) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, w)
}
