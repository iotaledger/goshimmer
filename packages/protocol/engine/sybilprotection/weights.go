package sybilprotection

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
)

type Weights struct {
	Events      *Events
	store       kvstore.KVStore
	settings    *permanent.Settings
	weights     *ads.Map[identity.ID, Weight, *identity.ID, *Weight]
	totalWeight *Weight
	mutex       sync.RWMutex
}

func NewWeights(store kvstore.KVStore, settings *permanent.Settings) (newConsensusWeights *Weights) {
	return &Weights{
		Events:   NewEvents(),
		store:    store,
		settings: settings,
		weights:  ads.NewMap[identity.ID, Weight](store),
	}
}

func (w *Weights) Weight(id identity.ID) (weight *Weight, exists bool) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Get(id)
}

func (w *Weights) ApplyUpdates(updates *WeightUpdates) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	direction := int64(lo.Compare(updates.TargetEpoch(), w.totalWeight.UpdateTime))
	removedWeights := set.NewAdvancedSet[identity.ID]()

	updates.ForEach(func(id identity.ID, diff int64) {
		oldWeight, exists := w.Weight(id)
		if !exists {
			oldWeight = NewWeight(0, -1)
		} else if updates.TargetEpoch() == oldWeight.UpdateTime {
			if oldWeight.Value == 0 {
				removedWeights.Add(id)
			}

			return
		}

		// We will subtract if the "direction" returned by the Compare is negative, effectively rolling back.
		newWeight := oldWeight.Value + direction*diff
		if newWeight == 0 {
			removedWeights.Add(id)
		}

		w.weights.Set(id, NewWeight(newWeight, updates.TargetEpoch()))
	})

	w.totalWeight.Value += direction * updates.TotalDiff()

	w.Events.WeightsUpdated.Trigger(updates)

	// after written to disk (remove deleted elements)
	_ = removedWeights.ForEach(func(id identity.ID) error {
		w.weights.Delete(id)
		return nil
	})

	// mark as clean
}

func (w *Weights) NewWeightedSet(members ...identity.ID) *WeightedSet {
	return NewWeightedSet(w, members...)
}

func (w *Weights) Root() types.Identifier {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Root()
}

func (w *Weights) TotalWeight() int64 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.totalWeight.Value
}

// Stream streams the weights of all actors and when they were last updated.
func (w *Weights) Stream(callback func(id identity.ID, weight *Weight) bool) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Stream(callback)
}

func (w *Weights) Map() (weights map[identity.ID]int64) {
	weights = make(map[identity.ID]int64)
	if err := w.Stream(func(id identity.ID, weight *Weight) bool {
		weights[id] = weight.Value
		return true
	}); err != nil {
		panic(err)
	}

	return weights
}
