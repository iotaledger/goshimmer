package pos

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
)

type WeightsVector struct {
}

func (w WeightsVector) Weight(identities ...identity.ID) uint64 {
	// TODO implement me
	panic("implement me")
}

func (w WeightsVector) SetWeight(id identity.ID, weight uint64) (updated bool) {
	// TODO implement me
	panic("implement me")
}

func (w WeightsVector) NewWeightedSet(members ...identity.ID) sybilprotection.WeightedSet {
	// TODO implement me
	panic("implement me")
}

func (w WeightsVector) SubscribeWeightUpdates(callback func(id identity.ID, oldValue uint64, newValue uint64), optSync ...bool) (cancel func()) {
	closure := event.NewClosure(func(err error) {
		callback(identity.ID{}, 0, 0)
	})

	return func() {
		closure.Detach()
	}
}

func (w WeightsVector) TotalWeight() uint64 {
	// TODO implement me
	panic("implement me")
}

var _ sybilprotection.WeightsVector = &WeightsVector{}
