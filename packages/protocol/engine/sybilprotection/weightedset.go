package sybilprotection

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
)

type WeightedSet struct {
	Weights              *Weights
	weightUpdatesClosure *event.Closure[*WeightsBatch]
	members              *set.AdvancedSet[identity.ID]
	membersMutex         sync.RWMutex
	totalWeight          int64
	totalWeightMutex     sync.RWMutex
}

func NewWeightedSet(weights *Weights, optMembers ...identity.ID) (newWeightedSet *WeightedSet) {
	newWeightedSet = new(WeightedSet)
	newWeightedSet.Weights = weights
	newWeightedSet.weightUpdatesClosure = event.NewClosure(newWeightedSet.onWeightUpdated)
	newWeightedSet.members = set.NewAdvancedSet[identity.ID]()

	//weights.Events.WeightsUpdated.Attach(newWeightedSet.weightUpdatesClosure)

	for _, member := range optMembers {
		newWeightedSet.Add(member)
	}

	return
}

func (w *WeightedSet) Add(id identity.ID) (added bool) {
	w.Weights.mutex.RLock()
	defer w.Weights.mutex.RUnlock()

	w.membersMutex.Lock()
	defer w.membersMutex.Unlock()

	if added = w.members.Add(id); added {
		if weight, exists := w.Weights.get(id); exists {
			w.totalWeight += weight.Value
		}
	}

	return
}

func (w *WeightedSet) Delete(id identity.ID) (removed bool) {
	w.Weights.mutex.RLock()
	defer w.Weights.mutex.RUnlock()

	w.membersMutex.Lock()
	defer w.membersMutex.Unlock()

	if removed = w.members.Delete(id); removed {
		if weight, exists := w.Weights.get(id); exists {
			w.totalWeight -= weight.Value
		}
	}

	return
}

func (w *WeightedSet) Get(id identity.ID) (weight *Weight, exists bool) {
	w.membersMutex.RLock()
	defer w.membersMutex.RUnlock()

	if !w.members.Has(id) {
		return nil, false
	}

	if weight, exists = w.Weights.Get(id); exists {
		return weight, true
	}

	return NewWeight(0, -1), true
}

func (w *WeightedSet) Has(id identity.ID) (has bool) {
	w.membersMutex.RLock()
	defer w.membersMutex.RUnlock()

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

func (w *WeightedSet) Members() *set.AdvancedSet[identity.ID] {
	w.membersMutex.RLock()
	defer w.membersMutex.RUnlock()

	return w.members
}

func (w *WeightedSet) Detach() {
	//w.Weights.Events.WeightsUpdated.Detach(w.weightUpdatesClosure)
}

func (w *WeightedSet) onWeightUpdated(updates *WeightsBatch) {
	w.totalWeightMutex.Lock()
	defer w.totalWeightMutex.Unlock()

	updates.ForEach(func(id identity.ID, diff int64) {
		if w.members.Has(id) {
			w.totalWeight += diff
		}
	})
}
