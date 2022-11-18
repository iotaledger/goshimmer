package weights

import (
	"sync"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/storage/models"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
)

type Vector struct {
	Events *Events

	weights          *permanent.ConsensusWeights
	settings         *permanent.Settings
	totalWeight      int64
	totalWeightMutex sync.RWMutex
	mutex            syncutils.DAGMutex[identity.ID]
}

func NewVector(weightsStorage *permanent.ConsensusWeights, settingsStorage *permanent.Settings) (newVector *Vector) {
	return &Vector{
		Events:   NewEvents(),
		weights:  weightsStorage,
		settings: settingsStorage,
	}
}

func (v *Vector) Weight(id identity.ID) (weight int64) {
	v.mutex.RLock(id)
	defer v.mutex.RUnlock(id)

	return v.weight(id)
}

func (v *Vector) SetWeight(id identity.ID, newWeight int64) (updated bool, oldWeight int64) {
	if updated, oldWeight = v.updateWeight(id, newWeight); updated {
		v.Events.WeightUpdated.Trigger(&WeightUpdatedEvent{
			ID:       id,
			OldValue: oldWeight,
			NewValue: newWeight,
		})
	}

	return
}

func (v *Vector) NewWeightedSet(members ...identity.ID) *Set {
	return NewSet(v, members...)
}

func (v *Vector) TotalWeight() int64 {
	v.totalWeightMutex.RLock()
	defer v.totalWeightMutex.RUnlock()

	return v.totalWeight
}

func (v *Vector) weight(id identity.ID) (weight int64) {
	if timedBalance, exists := v.weights.Load(id); !exists {
		return 0
	} else {
		return timedBalance.Balance
	}
}

func (v *Vector) updateWeight(id identity.ID, newWeight int64) (updated bool, oldWeight int64) {
	v.mutex.Lock(id)
	defer v.mutex.Unlock(id)

	if oldWeight = v.weight(id); oldWeight == newWeight {
		return
	}

	if newWeight == 0 {
		v.weights.Delete(id)
	} else {
		v.weights.Store(id, &models.TimedBalance{
			Balance: newWeight,
			// we update the weights before committing (hence +1) as the commitment requires the updated root of the weights
			LastUpdated: v.settings.LatestCommitment().Index() + 1,
		})
	}

	v.updateTotalWeight(newWeight - oldWeight)

	return
}

func (v *Vector) updateTotalWeight(diff int64) {
	v.totalWeightMutex.Lock()
	defer v.totalWeightMutex.Unlock()

	v.totalWeight += diff
}
