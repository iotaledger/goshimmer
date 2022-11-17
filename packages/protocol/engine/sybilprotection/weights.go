package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/impl"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/types"
)

type Weights struct {
}

func (w Weights) Weight(identities ...identity.ID) uint64 {
	// TODO implement me
	panic("implement me")
}

func (w Weights) SetWeight(id identity.ID, weight uint64) {
	// TODO implement me
	panic("implement me")
}

func (w Weights) NewWeightedSet(members ...identity.ID) *impl.WeightedSet {
	// TODO implement me
	panic("implement me")
}

func (w Weights) Subscribe(callback func(id identity.ID, oldValue uint64, newValue uint64), optSync ...bool) *types.Subscription {
	// TODO implement me
	panic("implement me")
}

func (w Weights) TotalWeight() uint64 {
	// TODO implement me
	panic("implement me")
}

var _ types.WeightedActors = &Weights{}
