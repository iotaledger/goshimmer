package sybilprotection

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/zyedidia/generic/cache"

	"github.com/iotaledger/goshimmer/packages/core/ads"
)

const cacheSize = 1000

// Weights is a mapping between a collection of identities and their weights.
type Weights struct {
	// Events is a collection of events related to the Weights.
	Events *Events

	weights      *ads.Map[identity.ID, Weight, *identity.ID, *Weight]
	weightsCache *cache.Cache[identity.ID, *Weight]
	cacheMutex   sync.RWMutex
	totalWeight  *Weight
	mutex        sync.RWMutex
}

// NewWeights creates a new Weights instance.
func NewWeights(store kvstore.KVStore) (newWeights *Weights) {
	return &Weights{
		Events:       NewEvents(),
		weights:      ads.NewMap[identity.ID, Weight](store),
		weightsCache: cache.New[identity.ID, *Weight](cacheSize),
		totalWeight:  NewWeight(0, -1),
	}
}

// NewWeightedSet creates a new WeightedSet instance, that maintains a correct and updated total weight of its members.
func (w *Weights) NewWeightedSet(members ...identity.ID) (newWeightedSet *WeightedSet) {
	return NewWeightedSet(w, members...)
}

// Get returns the weight of the given identity.
func (w *Weights) Get(id identity.ID) (weight *Weight, exists bool) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.get(id)
}

// Update updates the weight of the given identity.
func (w *Weights) Update(id identity.ID, weightDiff *Weight) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if oldWeight, exists := w.weights.Get(id); exists {
		if newWeight := oldWeight.Value + weightDiff.Value; newWeight == 0 {
			w.weightsCache.Remove(id)
			w.weights.Delete(id)
		} else {
			w.weightsCache.Remove(id)
			w.weights.Set(id, NewWeight(newWeight, oldWeight.UpdateTime.Max(weightDiff.UpdateTime)))
		}
	} else {
		w.weightsCache.Remove(id)
		w.weights.Set(id, weightDiff)
	}

	w.totalWeight.Value += weightDiff.Value
}

// BatchUpdate updates the weights of multiple identities at once.
func (w *Weights) BatchUpdate(batch *WeightsBatch) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	direction := int64(lo.Compare(batch.TargetEpoch(), w.totalWeight.UpdateTime))
	removedWeights := set.NewAdvancedSet[identity.ID]()

	batch.ForEach(func(id identity.ID, diff int64) {
		oldWeight, exists := w.get(id)
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

		w.weightsCache.Remove(id)
		w.weights.Set(id, NewWeight(newWeight, batch.TargetEpoch()))
	})

	w.Events.WeightsUpdated.Trigger(batch)

	// after written to disk (remove deleted elements)
	_ = removedWeights.ForEach(func(id identity.ID) error {
		w.weightsCache.Remove(id)
		w.weights.Delete(id)
		return nil
	})

	w.totalWeight.Value += direction * batch.TotalDiff()
	w.totalWeight.UpdateTime = batch.TargetEpoch()

	// TODO: mark as clean
}

// ForEach iterates over all weights and calls the given callback for each of them.
func (w *Weights) ForEach(callback func(id identity.ID, weight *Weight) bool) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Stream(callback)
}

// TotalWeight returns the total weight of all identities.
func (w *Weights) TotalWeight() (totalWeight *Weight) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.totalWeight
}

// Root returns the root of the merkle tree of the stored weights.
func (w *Weights) Root() (root types.Identifier) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Root()
}

// Map returns the weights as a map.
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

func (w *Weights) get(id identity.ID) (weight *Weight, exists bool) {
	if weight, exists = w.getFromCache(id); exists {
		return weight, exists
	}

	weight, exists = w.weights.Get(id)
	if exists {
		w.setCache(id, weight)
	}

	return weight, exists
}

func (w *Weights) getFromCache(id identity.ID) (weight *Weight, exists bool) {
	w.cacheMutex.RLock()
	defer w.cacheMutex.RUnlock()

	return w.weightsCache.Get(id)
}

func (w *Weights) setCache(id identity.ID, weight *Weight) {
	w.cacheMutex.Lock()
	defer w.cacheMutex.Unlock()

	w.weightsCache.Put(id, weight)
}
