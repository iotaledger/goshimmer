package sybilprotection

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/zyedidia/generic/cache"

	"github.com/iotaledger/hive.go/ads"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
)

const cacheSize = 1000

// Weights is a mapping between a collection of identities and their weights.
type Weights struct {
	// Events is a collection of events related to the Weights.
	Events *Events

	weights      *ads.Map[identity.ID, Weight, *identity.ID, *Weight]
	weightsCache *cache.Cache[identity.ID, *Weight]
	cacheMutex   sync.Mutex
	totalWeight  *Weight
	mutex        sync.RWMutex
}

// NewWeights creates a new Weights instance.
func NewWeights(store kvstore.KVStore) (newWeights *Weights) {
	newWeights = &Weights{
		Events:       NewEvents(),
		weights:      ads.NewMap[identity.ID, Weight](store),
		weightsCache: cache.New[identity.ID, *Weight](cacheSize),
		totalWeight:  NewWeight(0, -1),
	}

	if err := newWeights.weights.Stream(func(_ identity.ID, value *Weight) bool {
		newWeights.totalWeight.Value += value.Value
		newWeights.totalWeight.UpdateTime = value.UpdateTime.Max(newWeights.totalWeight.UpdateTime)
		return true
	}); err != nil {
		return nil
	}

	return
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

	direction := int64(lo.Compare(batch.TargetSlot(), w.totalWeight.UpdateTime))
	removedWeights := advancedset.New[identity.ID]()

	batch.ForEach(func(id identity.ID, diff int64) {
		oldWeight, exists := w.get(id)
		if !exists {
			oldWeight = NewWeight(0, -1)
		} else if batch.TargetSlot() == oldWeight.UpdateTime {
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
		w.weights.Set(id, NewWeight(newWeight, batch.TargetSlot()))
	})

	w.Events.WeightsUpdated.Trigger(batch)

	// after written to disk (remove deleted elements)
	_ = removedWeights.ForEach(func(id identity.ID) error {
		w.weightsCache.Remove(id)
		w.weights.Delete(id)
		return nil
	})

	w.totalWeight.Value += direction * batch.TotalDiff()
	w.totalWeight.UpdateTime = batch.TargetSlot()

	// TODO: mark as clean
}

// ForEach iterates over all weights and calls the given callback for each of them.
func (w *Weights) ForEach(callback func(id identity.ID, weight *Weight) bool) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.weights.Stream(callback)
}

// TotalWeight returns the total weight of all identities.
func (w *Weights) TotalWeight() (totalWeight int64) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.totalWeight.Value
}

// TotalWeightWithoutZeroIdentity returns the total weight of all identities minus the zero identity.
func (w *Weights) TotalWeightWithoutZeroIdentity() int64 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	var totalWeight int64
	if zeroIdentityWeight, exists := w.get(identity.ID{}); exists {
		totalWeight -= zeroIdentityWeight.Value
	}

	return totalWeight + w.totalWeight.Value
}

func (w *Weights) UpdateTotalWeightSlot(index slot.Index) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.totalWeight.UpdateTime = index
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
		return nil, errors.Wrap(err, "failed to export weights")
	}

	return weights, nil
}

func (w *Weights) get(id identity.ID) (weight *Weight, exists bool) {
	if weight, exists = w.getFromCache(id); exists {
		if weight.UpdateTime == -1 {
			return weight, false
		}
		return weight, exists
	}

	weight, exists = w.weights.Get(id)
	if !exists {
		weight = NewWeight(0, -1)
	}
	w.setCache(id, weight)

	return weight, exists
}

func (w *Weights) getFromCache(id identity.ID) (weight *Weight, exists bool) {
	w.cacheMutex.Lock()
	defer w.cacheMutex.Unlock()

	return w.weightsCache.Get(id)
}

func (w *Weights) setCache(id identity.ID, weight *Weight) {
	w.cacheMutex.Lock()
	defer w.cacheMutex.Unlock()

	w.weightsCache.Put(id, weight)
}
