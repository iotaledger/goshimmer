package weight

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/stringify"
)

// Weight represents a mutable multi-tiered weight value that can be updated in-place.
type Weight struct {
	// OnUpdate is an event that is triggered when the weight value is updated.
	OnUpdate *event.Event1[Value]

	// value is the current weight Value.
	value Value

	// validators is the set of validators that are contributing to the validators weight.
	validators *sybilprotection.WeightedSet

	// validatorsHook is the hook that is triggered when the validators weight is updated.
	validatorsHook *event.Hook[func(int64)]

	// mutex is used to synchronize access to the weight value.
	mutex sync.RWMutex
}

// New creates a new Weight instance.
func New() *Weight {
	return &Weight{
		OnUpdate: event.New1[Value](),
	}
}

// CumulativeWeight returns the cumulative weight of the Weight.
func (w *Weight) CumulativeWeight() int64 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.value.CumulativeWeight()
}

// SetCumulativeWeight sets the cumulative weight of the Weight and returns the Weight (for chaining).
func (w *Weight) SetCumulativeWeight(cumulativeWeight int64) *Weight {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.value.CumulativeWeight() != cumulativeWeight {
		w.value = w.value.SetCumulativeWeight(cumulativeWeight)
		w.OnUpdate.Trigger(w.value)
	}

	return w
}

// AddCumulativeWeight adds the given weight to the cumulative weight and returns the Weight (for chaining).
func (w *Weight) AddCumulativeWeight(delta int64) *Weight {
	if delta != 0 {
		w.mutex.Lock()
		defer w.mutex.Unlock()

		w.value = w.value.AddCumulativeWeight(delta)
		w.OnUpdate.Trigger(w.value)
	}

	return w
}

// RemoveCumulativeWeight removes the given weight from the cumulative weight and returns the Weight (for chaining).
func (w *Weight) RemoveCumulativeWeight(delta int64) *Weight {
	if delta != 0 {
		w.mutex.Lock()
		defer w.mutex.Unlock()

		w.value = w.value.RemoveCumulativeWeight(delta)
		w.OnUpdate.Trigger(w.value)
	}

	return w
}

// Validators returns the set of validators that are contributing to the validators weight.
func (w *Weight) Validators() *sybilprotection.WeightedSet {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.validators
}

// SetValidators sets the validators that are contributing to the weight and returns the Weight (for chaining).
func (w *Weight) SetValidators(validators *sybilprotection.WeightedSet) *Weight {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.validators == validators {
		return w
	}

	if w.validatorsHook != nil {
		w.validatorsHook.Unhook()
	}

	w.validators = validators
	if validators == nil {
		w.updateValidatorsWeight(0)

		return w
	}

	w.validatorsHook = w.validators.OnTotalWeightUpdated.Hook(func(totalWeight int64) {
		w.mutex.Lock()
		defer w.mutex.Unlock()

		w.updateValidatorsWeight(totalWeight)
	})
	w.updateValidatorsWeight(validators.TotalWeight())

	return w
}

// AcceptanceState returns the acceptance state of the weight.
func (w *Weight) AcceptanceState() acceptance.State {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.value.AcceptanceState()
}

// SetAcceptanceState sets the acceptance state of the weight and returns the previous acceptance state.
func (w *Weight) SetAcceptanceState(acceptanceState acceptance.State) (previousState acceptance.State) {
	if previousState = w.setAcceptanceState(acceptanceState); previousState != acceptanceState {
		w.OnUpdate.Trigger(w.value)
	}

	return previousState
}

// Value returns an immutable copy of the Weight.
func (w *Weight) Value() Value {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.value
}

// Compare compares the Weight to the given other Weight.
func (w *Weight) Compare(other *Weight) Comparison {
	switch {
	case w == nil && other == nil:
		return Equal
	case w == nil:
		return Heavier
	case other == nil:
		return Lighter
	default:
		return w.value.Compare(other.value)
	}
}

// String returns a human-readable representation of the Weight.
func (w *Weight) String() string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return stringify.Struct("Weight",
		stringify.NewStructField("Value", w.value),
		stringify.NewStructField("Validators", w.validators),
	)
}

// updateValidatorsWeight updates the validators weight of the Weight.
func (w *Weight) updateValidatorsWeight(weight int64) {
	if w.value.ValidatorsWeight() != weight {
		w.value = w.value.SetValidatorsWeight(weight)

		w.OnUpdate.Trigger(w.value)
	}
}

// setAcceptanceState sets the acceptance state of the weight and returns the previous acceptance state.
func (w *Weight) setAcceptanceState(acceptanceState acceptance.State) (previousState acceptance.State) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if previousState = w.value.AcceptanceState(); previousState != acceptanceState {
		fmt.Println("setAcceptanceState", acceptanceState)
		w.value = w.value.SetAcceptanceState(acceptanceState)
	}

	return previousState
}