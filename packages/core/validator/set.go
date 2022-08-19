package validator

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
)

type Set struct {
	validators  *shrinkingmap.ShrinkingMap[identity.ID, *Validator]
	totalWeight uint64

	mutex sync.RWMutex
}

func NewSet(validators ...*Validator) *Set {
	newSet := &Set{
		validators: shrinkingmap.New[identity.ID, *Validator](),
	}

	for _, validator := range validators {
		newSet.validators.Set(validator.ID(), validator)
		newSet.totalWeight += validator.Weight()

		validator.Events.WeightUpdated.Hook(event.NewClosure[*WeightUpdatedEvent](func(updatedEvent *WeightUpdatedEvent) {
			newSet.mutex.Lock()
			defer newSet.mutex.Unlock()

			newSet.totalWeight += updatedEvent.NewWeight - updatedEvent.OldWeight
		}))
	}

	return newSet
}

func (s *Set) TotalWeight() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.totalWeight
}

func (s *Set) Get(id identity.ID) (validator *Validator, exists bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.validators.Get(id)
}

func (s *Set) Add(validator *Validator) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.validators.Set(validator.ID(), validator)
}
