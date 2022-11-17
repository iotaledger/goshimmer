package pos

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
)

type WeightedSet struct {
	dispose     func()
	disposeOnce sync.Once
	source      sybilprotection.WeightsVector
	members     *set.AdvancedSet[identity.ID]
	totalWeight uint64
	mutex       sync.RWMutex
}

func NewWeightedSet(weightSource sybilprotection.WeightsVector) (weightedSet *WeightedSet) {
	weightedSet = new(WeightedSet)
	weightedSet.source = weightSource
	weightedSet.members = set.NewAdvancedSet[identity.ID]()
	weightedSet.dispose = weightSource.SubscribeWeightUpdates(weightedSet.onWeightUpdated, true)

	return
}

func (w *WeightedSet) Has(id identity.ID) (has bool) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.members.Has(id)
}

func (w *WeightedSet) Add(id identity.ID) (added bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if added = w.members.Add(id); added {
		w.totalWeight += w.source.Weight(id)
	}

	return
}

func (w *WeightedSet) Remove(id identity.ID) (removed bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if removed = w.members.Delete(id); removed {
		w.totalWeight -= w.source.Weight(id)
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

func (w *WeightedSet) TotalWeight() uint64 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.totalWeight
}

func (w *WeightedSet) Dispose() {
	w.disposeOnce.Do(w.dispose)
}

func (w *WeightedSet) onWeightUpdated(id identity.ID, oldValue uint64, newValue uint64) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.members.Has(id) {
		w.totalWeight += newValue - oldValue
	}
}

var _ sybilprotection.WeightedSet = &WeightedSet{}
