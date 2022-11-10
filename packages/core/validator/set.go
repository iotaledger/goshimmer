package validator

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

// region Set //////////////////////////////////////////////////////////////////////////////////////////////////////////

type Set struct {
	validators     *memstorage.Storage[identity.ID, *Validator]
	totalWeight    int64
	optTrackEvents bool
	eventClosures  *shrinkingmap.ShrinkingMap[identity.ID, *event.Closure[*WeightUpdatedEvent]]

	mutex sync.RWMutex
}

func NewSet(opts ...options.Option[Set]) *Set {
	return options.Apply(&Set{
		validators:    memstorage.New[identity.ID, *Validator](),
		eventClosures: shrinkingmap.New[identity.ID, *event.Closure[*WeightUpdatedEvent]](),
	}, opts)
}

func (s *Set) AddAll(validators ...*Validator) *Set {
	for _, validator := range validators {
		s.validators.Set(validator.ID(), validator)
		s.totalWeight += int64(validator.Weight())

		if s.optTrackEvents {
			eventClosure := event.NewClosure[*WeightUpdatedEvent](s.handleWeightUpdatedEvent)
			s.eventClosures.Set(validator.ID(), eventClosure)

			validator.Events.WeightUpdated.Hook(eventClosure)
		}
	}

	return s
}

func (s *Set) TotalWeight() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.totalWeight
}

func (s *Set) Get(id identity.ID) (validator *Validator, exists bool) {
	return s.validators.Get(id)
}

func (s *Set) Add(validator *Validator) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.validators.Set(validator.ID(), validator) {
		return
	}

	s.totalWeight += validator.Weight()

	if s.optTrackEvents {
		validator.Events.WeightUpdated.Hook(event.NewClosure[*WeightUpdatedEvent](s.handleWeightUpdatedEvent))
	}
}

func (s *Set) Delete(validator *Validator) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.validators.Delete(validator.ID())

	s.totalWeight -= validator.Weight()

	if s.optTrackEvents {
		closure, exists := s.eventClosures.Get(validator.ID())
		if exists {
			validator.Events.WeightUpdated.Detach(closure)
		}
	}
}

func (s *Set) handleWeightUpdatedEvent(updatedEvent *WeightUpdatedEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.totalWeight += int64(updatedEvent.NewWeight) - int64(updatedEvent.OldWeight)
}

// ForEachKey iterates through the map and calls the consumer for every element.
// Returning false from this function indicates to abort the iteration.
func (s *Set) ForEachKey(callback func(id identity.ID) bool) {
	s.validators.ForEachKey(callback)
}

// ForEach iterates through the map and calls the consumer for every element.
// Returning false from this function indicates to abort the iteration.
func (s *Set) ForEach(callback func(id identity.ID, validator *Validator) bool) {
	s.validators.ForEach(callback)
}

func (s *Set) Size() (size int) {
	return s.validators.Size()
}

func (s *Set) Slice() (validators []*Validator) {
	s.ForEach(func(id identity.ID, validator *Validator) bool {
		validators = append(validators, validator)
		return true
	})
	return
}

func (s *Set) IsThresholdReached(otherWeight int64, threshold float64) bool {
	return IsThresholdReached(s.TotalWeight(), otherWeight, threshold)
}

func IsThresholdReached(weight, otherWeight int64, threshold float64) bool {
	return otherWeight > int64(float64(weight)*threshold)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithValidatorEventTracking(trackEvents bool) options.Option[Set] {
	return func(set *Set) {
		set.optTrackEvents = trackEvents
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
