package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"
)

type WeightsVector interface {
	Weight(ids ...identity.ID) (weight uint64)
	SetWeight(id identity.ID, weight uint64) (updated bool)
	NewWeightedSet(ids ...identity.ID) WeightedSet
	TotalWeight() (totalWeight uint64)
	SubscribeWeightUpdates(callback func(id identity.ID, oldValue uint64, newValue uint64), optSync ...bool) (cancel func())
}
