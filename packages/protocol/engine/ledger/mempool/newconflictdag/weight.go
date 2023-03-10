package newconflictdag

import (
	"bytes"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/ds/types"
)

type Weight struct {
	id                types.Identifier
	cumulativeWeight  uint64
	validatorWeights  *sybilprotection.WeightedSet
	confirmationState ConfirmationState
}

func (w *Weight) Compare(other Weight) int {
	if result := w.compareConfirmationState(other); result != 0 {
		return result
	}

	if result := w.compareValidatorWeights(other); result != 0 {
		return result
	}

	if result := w.compareCumulativeWeight(other); result != 0 {
		return result
	}

	return bytes.Compare(w.id.Bytes(), other.id.Bytes())
}

func (w *Weight) compareConfirmationState(other Weight) int {
	if w.confirmationState == other.confirmationState {
		return 0
	}

	if w.confirmationState == Accepted {
		return 1
	}

	if w.confirmationState == Rejected {
		return -1
	}

	if other.confirmationState == Accepted {
		return -1
	}

	if other.confirmationState == Rejected {
		return 1
	}

	return 0
}

func (w *Weight) compareValidatorWeights(other Weight) int {
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

func (w *Weight) compareCumulativeWeight(other Weight) int {
	if w.cumulativeWeight < other.cumulativeWeight {
		return -1
	}

	if w.cumulativeWeight > other.cumulativeWeight {
		return 1
	}

	return 0
}
