package weight

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/hive.go/stringify"
)

type Value struct {
	cumulativeWeight int64
	validatorsWeight int64
	acceptanceState  acceptance.State
}

func NewValue(cumulativeWeight int64, validatorsWeight int64, acceptanceState acceptance.State) Value {
	return Value{
		cumulativeWeight: cumulativeWeight,
		validatorsWeight: validatorsWeight,
		acceptanceState:  acceptanceState,
	}
}

func (v Value) CumulativeWeight() int64 {
	return v.cumulativeWeight
}

func (v Value) AddCumulativeWeight(weight int64) Value {
	v.cumulativeWeight += weight

	return v
}

func (v Value) RemoveCumulativeWeight(weight int64) Value {
	v.cumulativeWeight -= weight

	return v
}

func (v Value) ValidatorsWeight() int64 {
	return v.validatorsWeight
}

func (v Value) SetValidatorsWeight(weight int64) Value {
	v.validatorsWeight = weight

	return v
}

func (v Value) AcceptanceState() acceptance.State {
	return v.acceptanceState
}

func (v Value) SetAcceptanceState(acceptanceState acceptance.State) Value {
	v.acceptanceState = acceptanceState

	return v
}

func (v Value) Compare(other Value) int {
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

func (v Value) String() string {
	return stringify.Struct("weight.Value",
		stringify.NewStructField("CumulativeWeight", v.cumulativeWeight),
		stringify.NewStructField("ValidatorsWeight", v.validatorsWeight),
		stringify.NewStructField("State", v.acceptanceState),
	)
}

func (v Value) compareConfirmationState(other Value) int {
	switch {
	case v.acceptanceState == acceptance.Accepted && other.acceptanceState != acceptance.Accepted:
		return Heavier
	case other.acceptanceState == acceptance.Rejected && v.acceptanceState != acceptance.Rejected:
		return Heavier
	case other.acceptanceState == acceptance.Accepted && v.acceptanceState != acceptance.Accepted:
		return Lighter
	case v.acceptanceState == acceptance.Rejected && other.acceptanceState != acceptance.Rejected:
		return Lighter
	default:
		return Equal
	}
}

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
