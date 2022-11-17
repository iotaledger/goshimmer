package types

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/generictypes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/impl"
)

type WeightedActors interface {
	Weight(ids ...identity.ID) (weight uint64)
	SetWeight(id identity.ID, weight uint64)
	NewWeightedSet(ids ...identity.ID) *impl.WeightedSet
	SubscribeWeightUpdates(callback func(id identity.ID, oldValue uint64, newValue uint64), optSync ...bool) *generictypes.Subscription
	TotalWeight() (totalWeight uint64)
}
