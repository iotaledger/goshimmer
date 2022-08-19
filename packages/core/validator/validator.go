package validator

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
)

type Validator struct {
	id     identity.ID
	weight uint64

	Events *ValidatorEvents

	mutex sync.RWMutex
}

func New(id identity.ID, opts ...options.Option[Validator]) *Validator {
	return options.Apply(&Validator{
		id:     id,
		weight: 1,
		Events: newValidatorEvents(),
	}, opts)
}

func (v *Validator) ID() identity.ID {
	return v.id
}

func (v *Validator) Weight() uint64 {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	return v.weight
}

func (v *Validator) SetWeight(weight uint64) (wasUpdated bool) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if oldWeight := v.weight; weight != oldWeight {
		v.weight = weight

		v.Events.WeightUpdated.Trigger(&WeightUpdatedEvent{
			Validator: v,
			OldWeight: oldWeight,
			NewWeight: weight,
		})

		return true
	}

	return false

}

func WithWeight(weight uint64) options.Option[Validator] {
	return func(validator *Validator) {
		validator.weight = weight
	}
}
