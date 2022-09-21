package validator

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

type ValidatorEvents struct {
	WeightUpdated *event.Event[*WeightUpdatedEvent]
}

type WeightUpdatedEvent struct {
	Validator *Validator
	OldWeight int64
	NewWeight int64
}

// newValidatorEvents creates a new ValidatorEvents instance.
func newValidatorEvents() *ValidatorEvents {
	return &ValidatorEvents{
		WeightUpdated: event.New[*WeightUpdatedEvent](),
	}
}
