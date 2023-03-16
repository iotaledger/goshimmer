package weight

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/stringify"
)

type Weight struct {
	OnUpdate *event.Event1[Value]

	value      Value
	validators *sybilprotection.WeightedSet

	mutex sync.RWMutex
}

func New(cumulativeWeight int64, validators *sybilprotection.WeightedSet, acceptanceState acceptance.State) *Weight {
	w := &Weight{
		OnUpdate:   event.New1[Value](),
		validators: validators,
	}

	if validators != nil {
		w.value = NewValue(cumulativeWeight, validators.TotalWeight(), acceptanceState)
		w.validators.OnTotalWeightUpdated.Hook(w.onValidatorsWeightUpdated)
	} else {
		w.value = NewValue(cumulativeWeight, 0, acceptanceState)
	}

	return w
}

func (w *Weight) AddCumulativeWeight(delta int64) {
	if delta == 0 {
		return
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.value = w.value.AddCumulativeWeight(delta)

	w.OnUpdate.Trigger(w.value)
}

func (w *Weight) RemoveCumulativeWeight(delta int64) {
	if delta == 0 {
		return
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.value = w.value.RemoveCumulativeWeight(delta)

	w.OnUpdate.Trigger(w.value)
}

func (w *Weight) AddValidator(id identity.ID) {
	w.validators.Add(id)
}

func (w *Weight) RemoveValidator(id identity.ID) {
	w.validators.Delete(id)
}

func (w *Weight) SetAcceptanceState(acceptanceState acceptance.State) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.value.AcceptanceState() == acceptanceState {
		return

	}

	w.value = w.value.SetAcceptanceState(acceptanceState)

	w.OnUpdate.Trigger(w.value)
}

func (w *Weight) Value() Value {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.value
}

func (w *Weight) Compare(other *Weight) int {
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

func (w *Weight) String() string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return stringify.Struct("weight.Weight",
		stringify.NewStructField("Value", w.value),
		stringify.NewStructField("Validators", w.validators),
	)
}

func (w *Weight) onValidatorsWeightUpdated(weight int64) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.value = w.value.SetValidatorsWeight(weight)

	w.OnUpdate.Trigger(w.value)
}
