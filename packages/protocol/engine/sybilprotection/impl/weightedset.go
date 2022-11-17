package impl

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/generictypes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/types"
)

type WeightedSet struct {
	source       types.WeightedActors
	members      *set.AdvancedSet[identity.ID]
	totalWeight  uint64
	mutex        sync.RWMutex
	subscription *generictypes.Subscription
}

func NewWeightedSet(weightSource types.WeightedActors) (weightedSet *WeightedSet) {
	weightedSet = new(WeightedSet)
	weightedSet.source = weightSource
	weightedSet.members = set.NewAdvancedSet[identity.ID]()
	weightedSet.subscription = weightSource.SubscribeWeightUpdates(weightedSet.onWeightUpdated, true)

	return
}

func (w *WeightedSet) Add(identities ...identity.ID) (addedIdentities *set.AdvancedSet[identity.ID]) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	addedIdentities = set.NewAdvancedSet[identity.ID]()
	for _, id := range identities {
		if w.members.Add(id) && addedIdentities.Add(id) {
			w.totalWeight += w.source.Weight(id)
		}
	}

	return
}

func (w *WeightedSet) Remove(identities ...identity.ID) (amountRemoved int) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, id := range identities {
		if w.members.Delete(id) {
			w.totalWeight -= w.source.Weight(id)

			amountRemoved++
		}
	}

	return
}

func (w *WeightedSet) ForEach(callback func(id identity.ID) error) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.members.ForEach(callback)
}

func (w *WeightedSet) ForEachWeighted(callback func(id identity.ID, weight uint64) error) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.members.ForEach(func(id identity.ID) error {
		return callback(id, w.source.Weight(id))
	})
}

func (w *WeightedSet) Weight() uint64 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.totalWeight
}

func (w *WeightedSet) Dispose() {
	w.subscription.Cancel()
}

func (w *WeightedSet) onWeightUpdated(id identity.ID, oldValue uint64, newValue uint64) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.members.Has(id) {
		w.totalWeight += newValue - oldValue
	}
}
