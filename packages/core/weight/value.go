package weight

import (
	"github.com/iotaledger/goshimmer/packages/core/acceptance"
	"github.com/iotaledger/hive.go/stringify"
)

// Value represents an immutable multi-tiered weight value, which is used to determine the order of Conflicts.
type Value struct {
	// cumulativeWeight is the lowest tier which accrues weight in a cumulative manner (i.e. PoW or burned mana).
	cumulativeWeight int64

	// validatorsWeight is the second tier which tracks weight in a non-cumulative manner (BFT style).
	validatorsWeight int64

	// acceptanceState is the final tier which determines the decision of the Conflict.
	acceptanceState acceptance.State
}

// CumulativeWeight returns the cumulative weight of the Value.
func (v Value) CumulativeWeight() int64 {
	return v.cumulativeWeight
}

// SetCumulativeWeight sets the cumulative weight of the Value and returns the new Value.
func (v Value) SetCumulativeWeight(cumulativeWeight int64) Value {
	v.cumulativeWeight = cumulativeWeight

	return v
}

// AddCumulativeWeight adds the given weight to the cumulative weight of the Value and returns the new Value.
func (v Value) AddCumulativeWeight(weight int64) Value {
	v.cumulativeWeight += weight

	return v
}

// RemoveCumulativeWeight removes the given weight from the cumulative weight of the Value and returns the new Value.
func (v Value) RemoveCumulativeWeight(weight int64) Value {
	v.cumulativeWeight -= weight

	return v
}

// ValidatorsWeight returns the weight of the validators.
func (v Value) ValidatorsWeight() int64 {
	return v.validatorsWeight
}

// SetValidatorsWeight sets the  weight of the validators and returns the new Value.
func (v Value) SetValidatorsWeight(weight int64) Value {
	v.validatorsWeight = weight

	return v
}

// AcceptanceState returns the acceptance state of the Value.
func (v Value) AcceptanceState() acceptance.State {
	return v.acceptanceState
}

// SetAcceptanceState sets the acceptance state of the Value and returns the new Value.
func (v Value) SetAcceptanceState(acceptanceState acceptance.State) Value {
	v.acceptanceState = acceptanceState

	return v
}

// Compare compares the Value to the given other Value and returns the result of the comparison.
func (v Value) Compare(other Value) Comparison {
	if result := v.compareConfirmationState(other); result != 0 {
		return result
	}

	if result := v.compareValidatorsWeight(other); result != 0 {
		return result
	}

	if result := v.compareCumulativeWeight(other); result != 0 {
		return result
	}

	return 0
}

// String returns a human-readable representation of the Value.
func (v Value) String() string {
	return stringify.Struct("Value",
		stringify.NewStructField("CumulativeWeight", v.cumulativeWeight),
		stringify.NewStructField("ValidatorsWeight", v.validatorsWeight),
		stringify.NewStructField("AcceptanceState", v.acceptanceState),
	)
}

// compareConfirmationState compares the confirmation state of the Value to the confirmation state of the other Value.
func (v Value) compareConfirmationState(other Value) int {
	switch {
	case v.acceptanceState.IsAccepted() && !other.acceptanceState.IsAccepted():
		return Heavier
	case other.acceptanceState.IsRejected() && !v.acceptanceState.IsRejected():
		return Heavier
	case other.acceptanceState.IsAccepted() && !v.acceptanceState.IsAccepted():
		return Lighter
	case v.acceptanceState.IsRejected() && !other.acceptanceState.IsRejected():
		return Lighter
	default:
		return Equal
	}
}

// compareValidatorsWeight compares the validators weight of the Value to the validators weight of the other Value.
func (v Value) compareValidatorsWeight(other Value) int {
	switch {
	case v.validatorsWeight > other.validatorsWeight:
		return Heavier
	case v.validatorsWeight < other.validatorsWeight:
		return Lighter
	default:
		return Equal
	}
}

// compareCumulativeWeight compares the cumulative weight of the Value to the cumulative weight of the other Value.
func (v Value) compareCumulativeWeight(other Value) int {
	switch {
	case v.cumulativeWeight > other.cumulativeWeight:
		return Heavier
	case v.cumulativeWeight < other.cumulativeWeight:
		return Lighter
	default:
		return Equal
	}
}
