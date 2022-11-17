package types

import (
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
)

type WeightedSet interface {
	Add(ids ...identity.ID) (addedIDs *set.AdvancedSet[identity.ID])
	Remove(ids ...identity.ID) (removedIDs *set.AdvancedSet[identity.ID])
	Has(id identity.ID) (has bool)
	ForEach(callback func(id identity.ID) error) (err error)
	ForEachWeighted(callback func(id identity.ID, weight uint64) (err error)) (err error)
	TotalWeight() (totalWeight uint64)
	Dispose()
}
