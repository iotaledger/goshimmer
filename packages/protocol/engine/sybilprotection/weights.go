package sybilprotection

import (
	"sync"

	"github.com/cockroachdb/errors"
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
	settings    *permanent.Settings
	weights     *ads.Map[identity.ID, Weight, *identity.ID, *Weight]
	totalWeight *Weight
	mutex       sync.RWMutex
}

func NewWeights(store kvstore.KVStore, settings *permanent.Settings) (newConsensusWeights *Weights) {
	return &Weights{
		Events:      NewEvents(),
		settings:    settings,
		weights:     ads.NewMap[identity.ID, Weight](store),
		totalWeight: NewWeight(0, -1),
	}
}

func (w *Weights) NewWeightedSet(members ...identity.ID) *WeightedSet {
	return NewWeightedSet(w, members...)
}

func (w *Weights) Get(id identity.ID) (weight *Weight, exists bool) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Get(id)
}

func (w *Weights) Update(id identity.ID, diff *Weight) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if oldWeight, exists := w.weights.Get(id); exists {
		if newWeight := oldWeight.Value + diff.Value; newWeight == 0 {
			w.weights.Delete(id)
		} else {
			w.weights.Set(id, NewWeight(newWeight, oldWeight.UpdateTime.Max(diff.UpdateTime)))
		}
	} else {
		w.weights.Set(id, diff)
	}

	w.totalWeight.Value += diff.Value
}

func (w *Weights) BatchUpdate(batch *WeightsBatch) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	direction := int64(lo.Compare(batch.TargetEpoch(), w.totalWeight.UpdateTime))
	removedWeights := set.NewAdvancedSet[identity.ID]()

	batch.ForEach(func(id identity.ID, diff int64) {
		oldWeight, exists := w.weights.Get(id)
		if !exists {
			oldWeight = NewWeight(0, -1)
		} else if batch.TargetEpoch() == oldWeight.UpdateTime {
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

		w.weights.Set(id, NewWeight(newWeight, batch.TargetEpoch()))
	})

	w.Events.WeightsUpdated.Trigger(batch)

	// after written to disk (remove deleted elements)
	_ = removedWeights.ForEach(func(id identity.ID) error {
		w.weights.Delete(id)
		return nil
	})

	w.totalWeight.Value += direction * batch.TotalDiff()
	w.totalWeight.UpdateTime = batch.TargetEpoch()

	// TODO: mark as clean
}

func (w *Weights) ForEach(callback func(id identity.ID, weight *Weight) bool) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Stream(callback)
}

func (w *Weights) TotalWeight() (totalWeight *Weight) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.totalWeight
}

func (w *Weights) Root() (root types.Identifier) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Root()
}

func (w *Weights) Map() (weights map[identity.ID]int64, err error) {
	weights = make(map[identity.ID]int64)
	if err = w.ForEach(func(id identity.ID, weight *Weight) bool {
		weights[id] = weight.Value
		return true
	}); err != nil {
		return nil, errors.Errorf("failed to export weights: %w", err)
	}

	return weights, nil
}
