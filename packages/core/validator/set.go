package validator

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

type Set struct {
	validators  *memstorage.Storage[identity.ID, *Validator]
	totalWeight uint64

	mutex sync.RWMutex
}

func NewSet(validators ...*Validator) *Set {
	newSet := &Set{
		validators: memstorage.New[identity.ID, *Validator](),
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
	return s.validators.Get(id)
}

func (s *Set) Add(validator *Validator) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.validators.Set(validator.ID(), validator)

	s.totalWeight += validator.Weight()

	validator.Events.WeightUpdated.Hook(event.NewClosure[*WeightUpdatedEvent](func(updatedEvent *WeightUpdatedEvent) {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		s.totalWeight += updatedEvent.NewWeight - updatedEvent.OldWeight
	}))
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
