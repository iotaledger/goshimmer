package types

import (
	"time"

	"github.com/iotaledger/hive.go/core/identity"
)

type ActiveValidators interface {
	MarkActive(id identity.ID, timestamp time.Time)
	TotalWeight() (totalWeight uint64)
	ForEach(callback func(id identity.ID) error) error
	ForEachWeighted(callback func(id identity.ID, weight uint64) (err error)) (err error)
}
