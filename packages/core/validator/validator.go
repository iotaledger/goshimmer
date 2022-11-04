package validator

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
)

type Validator struct {
	id     identity.ID
	weight int64

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

func (v *Validator) Weight() int64 {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	return v.weight
}

func (v *Validator) SetWeight(weight int64) (wasUpdated bool) {
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

func (v *Validator) String() string {
	return fmt.Sprintf("Validator(%s, %d)", v.id.String(), v.weight)
}

func WithWeight(weight int64) options.Option[Validator] {
	return func(validator *Validator) {
		validator.weight = weight
	}
}
