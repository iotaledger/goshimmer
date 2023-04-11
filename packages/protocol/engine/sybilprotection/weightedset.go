package sybilprotection

import (
	"sync"

	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

type WeightedSet struct {
	OnTotalWeightUpdated *event.Event1[int64]

	Weights             *Weights
	weightUpdatesDetach *event.Hook[func(*WeightsBatch)]
	members             *advancedset.AdvancedSet[identity.ID]
	totalWeight         int64
	totalWeightMutex    sync.RWMutex
}

func NewWeightedSet(weights *Weights, optMembers ...identity.ID) *WeightedSet {
	w := &WeightedSet{
		OnTotalWeightUpdated: event.New1[int64](),
		Weights:              weights,
		members:              advancedset.New[identity.ID](),
	}
	w.weightUpdatesDetach = weights.Events.WeightsUpdated.Hook(w.applyWeightUpdates)

	w.Weights.mutex.RLock()
	defer w.Weights.mutex.RUnlock()

	for _, member := range optMembers {
		if w.members.Add(member) {
			w.totalWeight += lo.Return1(w.Weights.get(member)).Value
		}
	}

	return w
}

func (w *WeightedSet) Add(id identity.ID) (added bool) {
	w.Weights.mutex.RLock()
	defer w.Weights.mutex.RUnlock()

	if added = w.members.Add(id); added {
		if weight, exists := w.Weights.get(id); exists {
			w.totalWeightMutex.Lock()
			defer w.totalWeightMutex.Unlock()

			w.totalWeight += weight.Value

			if weight.Value != 0 {
				w.OnTotalWeightUpdated.Trigger(w.totalWeight)
			}
		}
	}

	return
}

func (w *WeightedSet) Delete(id identity.ID) (removed bool) {
	w.Weights.mutex.RLock()
	defer w.Weights.mutex.RUnlock()

	if removed = w.members.Delete(id); removed {
		if weight, exists := w.Weights.get(id); exists {
			w.totalWeightMutex.Lock()
			defer w.totalWeightMutex.Unlock()

			w.totalWeight -= weight.Value

			if weight.Value != 0 {
				w.OnTotalWeightUpdated.Trigger(w.totalWeight)
			}
		}
	}

	return
}

func (w *WeightedSet) Get(id identity.ID) (weight *Weight, exists bool) {
	if !w.members.Has(id) {
		return nil, false
	}

	if weight, exists = w.Weights.Get(id); exists {
		return weight, true
	}

	return NewWeight(0, -1), true
}

func (w *WeightedSet) Has(id identity.ID) (has bool) {
	return w.members.Has(id)
}

func (w *WeightedSet) ForEach(callback func(id identity.ID) error) (err error) {
	for it := w.members.Iterator(); it.HasNext(); {
		member := it.Next()
		if err = callback(member); err != nil {
			return
		}
	}

	return
}

func (w *WeightedSet) ForEachWeighted(callback func(id identity.ID, weight int64) error) (err error) {
	for it := w.members.Iterator(); it.HasNext(); {
		member := it.Next()
		memberWeight, exists := w.Weights.Get(member)
		if !exists {
			memberWeight = NewWeight(0, -1)
		}
		if err = callback(member, memberWeight.Value); err != nil {
			return
		}
	}

	return
}

func (w *WeightedSet) TotalWeight() (totalWeight int64) {
	w.totalWeightMutex.RLock()
	defer w.totalWeightMutex.RUnlock()

	return w.totalWeight
}

func (w *WeightedSet) Detach() {
	w.weightUpdatesDetach.Unhook()
}

func (w *WeightedSet) String() string {
	return w.members.String()
}

func (w *WeightedSet) applyWeightUpdates(updates *WeightsBatch) {
	w.totalWeightMutex.Lock()
	defer w.totalWeightMutex.Unlock()

	newWeight := w.totalWeight
	updates.ForEach(func(id identity.ID, diff int64) {
		if w.members.Has(id) {
			newWeight += diff
		}
	})

	if newWeight != w.totalWeight {
		w.totalWeight = newWeight

		w.OnTotalWeightUpdated.Trigger(newWeight)
	}
}
