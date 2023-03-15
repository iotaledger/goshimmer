package newconflictdag

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/stringify"
)

type Weight struct {
	OnUpdate *event.Event

	cumulativeWeight  uint64
	validatorWeights  *sybilprotection.WeightedSet
	confirmationState ConfirmationState

	mutex sync.RWMutex
}

func NewWeight(cumulativeWeight uint64, validatorWeights *sybilprotection.WeightedSet, confirmationState ConfirmationState) *Weight {
	return &Weight{
		OnUpdate: event.New(),

		cumulativeWeight:  cumulativeWeight,
		validatorWeights:  validatorWeights,
		confirmationState: confirmationState,
	}
}

func (w *Weight) AddCumulativeWeight(delta uint64) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if delta != 0 {
		w.cumulativeWeight += delta

		w.OnUpdate.Trigger()
	}
}

func (w *Weight) RemoveCumulativeWeight(delta uint64) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if delta != 0 {
		w.cumulativeWeight -= delta

		w.OnUpdate.Trigger()
	}
}

func (w *Weight) SetConfirmationState(confirmationState ConfirmationState) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.confirmationState != confirmationState {
		w.confirmationState = confirmationState

		w.OnUpdate.Trigger()
	}
}

func (w *Weight) Compare(other *Weight) int {
	if result := w.compareConfirmationState(other); result != 0 {
		return result
	}

	if result := w.compareValidatorWeights(other); result != 0 {
		return result
	}

	if result := w.compareCumulativeWeight(other); result != 0 {
		return result
	}

	return 0
}

func (w *Weight) String() string {
	return stringify.Struct("Weight",
		stringify.NewStructField("cumulativeWeight", w.cumulativeWeight),
		stringify.NewStructField("confirmationState", w.confirmationState),
	)
}

func (w *Weight) compareConfirmationState(other *Weight) int {
	if w.confirmationState != other.confirmationState {
		if w.confirmationState == Accepted {
			return 1
		}

		if other.confirmationState == Accepted {
			return -1
		}

		if w.confirmationState == Rejected {
			return -1
		}

		if other.confirmationState == Rejected {
			return 1
		}
	}

	return 0
}

func (w *Weight) compareValidatorWeights(other *Weight) int {
	if w.validatorWeights == nil && other.validatorWeights == nil {
		return 0
	}

	if w.validatorWeights == nil {
		return -1
	}

	if other.validatorWeights == nil {
		return 1
	}

	if w.validatorWeights.TotalWeight() > other.validatorWeights.TotalWeight() {
		return 1
	}

	if w.validatorWeights.TotalWeight() < other.validatorWeights.TotalWeight() {
		return -1
	}

	return 0
}

func (w *Weight) compareCumulativeWeight(other *Weight) int {
	if w.cumulativeWeight < other.cumulativeWeight {
		return -1
	}

	if w.cumulativeWeight > other.cumulativeWeight {
		return 1
	}

	return 0
}
