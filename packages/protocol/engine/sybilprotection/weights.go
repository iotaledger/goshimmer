package sybilprotection

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
)

type Weights struct {
	Events           *Events
	store            kvstore.KVStore
	settings         *permanent.Settings
	weights          *ads.Map[identity.ID, Weight, *Weight]
	totalWeight      int64
	totalWeightMutex sync.RWMutex
	mutex            syncutils.DAGMutex[identity.ID]
}

func NewWeights(store kvstore.KVStore, settings *permanent.Settings) (newConsensusWeights *Weights) {
	return &Weights{
		Events:   NewEvents(),
		store:    store,
		settings: settings,
		weights:  ads.NewMap[identity.ID, Weight](store),
	}
}

func (w *Weights) SetWeight(id identity.ID, weight *Weight) (updated bool, previousWeight *Weight) {
	if updated, previousWeight = w.setWeight(id, weight); updated {
		w.Events.WeightsUpdated.Trigger(&WeightUpdatedEvent{
			ID:        id,
			OldWeight: previousWeight,
			NewWeight: weight,
		})
	}

	return
}

func (w *Weights) Weight(id identity.ID) (weight *Weight, exists bool) {
	w.mutex.RLock(id)
	defer w.mutex.RUnlock(id)

	return w.weights.Get(id)
}

func (w *Weights) Apply(updates *WeightUpdates) {
	updates.ForEach(func(id identity.ID, diff int64) {
		oldWeight, exists := w.Weight(id)
		if !exists {
			oldWeight = NewWeight(0, -1)
		} else if updates.TargetEpoch() == oldWeight.UpdateTime {
			return
		}

		// We will subtract if the "direction" returned by the Compare is negative, effectively rolling back.
		if newWeight := oldWeight.Value + diff*int64(lo.Compare(updates.TargetEpoch(), oldWeight.UpdateTime)); newWeight != 0 {
			w.weights.Set(id, NewWeight(newWeight, updates.TargetEpoch()))
		} else {
			w.weights.Delete(id)
		}
	})

	w.totalWeight += updates.TotalDiff()

	w.Events.WeightsUpdated.Trigger(updates)
}

func (w *Weights) NewWeightedSet(members ...identity.ID) *WeightedSet {
	return NewWeightedSet(w, members...)
}

func (w *Weights) Root() types.Identifier {
	return w.weights.Root()
}

// Stream streams the weights of all actors and when they were last updated.
func (w *Weights) Stream(callback func(id identity.ID, weight int64, lastUpdated epoch.Index) bool) (err error) {
	if storageErr := w.store.Iterate([]byte{}, func(idBytes kvstore.Key, timedBalanceBytes kvstore.Value) bool {
		var id identity.ID
		if _, err = id.FromBytes(idBytes); err != nil {
			return false
		}

		timedBalance := new(Weight)
		if _, err = timedBalance.FromBytes(timedBalanceBytes); err != nil {
			return false
		}

		return callback(id, timedBalance.Value, timedBalance.UpdateTime)
	}); storageErr != nil {
		return errors.Errorf("failed to iterate over Consensus Weights: %w", storageErr)
	}

	return err
}

func (w *Weights) setWeight(id identity.ID, weight *Weight) (updated bool, previousWeight *Weight) {
	w.mutex.Lock(id)
	defer w.mutex.Unlock(id)

	previousWeight, exists := w.weights.Get(id)
	if updated = !exists || previousWeight.UpdateTime < weight.UpdateTime; !updated {
		return
	}

	if weight.Value == 0 {
		w.weights.Delete(id)
	} else {
		w.weights.Set(id, weight)
	}

	w.updateTotalWeight(weight.Value - previousWeight.Value)

	return
}

func (w *Weights) TotalWeight() int64 {
	w.totalWeightMutex.RLock()
	defer w.totalWeightMutex.RUnlock()

	return w.totalWeight
}

func (w *Weights) AsMap() (weights map[identity.ID]int64) {
	weights = make(map[identity.ID]int64)
	if err := w.Stream(func(id identity.ID, weight int64, _ epoch.Index) bool {
		weights[id] = weight
		return true
	}); err != nil {
		panic(err)
	}

	return weights
}

func (w *Weights) updateTotalWeight(diff int64) {
	w.totalWeightMutex.Lock()
	defer w.totalWeightMutex.Unlock()

	w.totalWeight += diff
}
