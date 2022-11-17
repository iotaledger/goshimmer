package sybilprotection

import (
	"time"

	"github.com/iotaledger/hive.go/core/identity"
)

type ActiveValidators interface {
	MarkActive(id identity.ID, timestamp time.Time)
	Get(id identity.ID) (weight int64, exists bool)
	Has(id identity.ID) (has bool)
	TotalWeight() (totalWeight int64)
	ForEach(callback func(id identity.ID) error) error
	ForEachWeighted(callback func(id identity.ID, weight int64) (err error)) (err error)
}
