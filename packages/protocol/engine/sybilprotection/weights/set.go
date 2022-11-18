package weights

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
)

type Set struct {
	members              *set.AdvancedSet[identity.ID]
	totalWeight          int64
	vector               *Vector
	weightUpdatesClosure *event.Closure[*WeightUpdatedEvent]
	mutex                sync.RWMutex
}

func NewSet(vector *Vector, optMembers ...identity.ID) (newSet *Set) {
	newSet = new(Set)
	newSet.members = set.NewAdvancedSet[identity.ID]()
	newSet.vector = vector
	newSet.weightUpdatesClosure = event.NewClosure(newSet.onWeightUpdated)

	vector.Events.WeightUpdated.Attach(newSet.weightUpdatesClosure)

	for _, member := range optMembers {
		newSet.Add(member)
	}

	return
}

func (w *Set) Get(id identity.ID) (weight int64, exists bool) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	weight = w.vector.Weight(id)

	return weight, weight != 0
}

func (w *Set) Has(id identity.ID) (has bool) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.members.Has(id)
}

func (w *Set) Add(id identity.ID) (added bool) {
	w.vector.mutex.RLock(id)
	defer w.vector.mutex.RUnlock(id)

	w.mutex.Lock()
	defer w.mutex.Unlock()

	if added = w.members.Add(id); added {
		w.totalWeight += w.vector.weight(id)
	}

	return
}

func (w *Set) Delete(id identity.ID) (removed bool) {
	w.vector.mutex.RLock(id)
	defer w.vector.mutex.RUnlock(id)

	w.mutex.Lock()
	defer w.mutex.Unlock()

	if removed = w.members.Delete(id); removed {
		w.totalWeight -= w.vector.weight(id)
	}

	return
}

func (w *Set) ForEach(callback func(id identity.ID) error) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.members.ForEach(callback)
}

func (w *Set) ForEachWeighted(callback func(id identity.ID, weight int64) error) (err error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.members.ForEach(func(id identity.ID) error {
		return callback(id, w.vector.Weight(id))
	})
}

func (w *Set) Weight() (totalWeight int64) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.totalWeight
}

func (w *Set) Detach() {
	w.vector.Events.WeightUpdated.Detach(w.weightUpdatesClosure)
}

func (w *Set) onWeightUpdated(event *WeightUpdatedEvent) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.members.Has(event.ID) {
		w.totalWeight += event.NewValue - event.OldValue
	}
}
