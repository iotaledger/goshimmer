package ledger

import (
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type AvailabilityManager struct {
	TransactionSolidEvent *event.Event[*TransactionSolidEvent]

	*Ledger
}

func NewAvailabilityManager(ledger *Ledger) (newAvailabilityManager *AvailabilityManager) {
	newAvailabilityManager = &AvailabilityManager{
		TransactionSolidEvent: event.New[*TransactionSolidEvent](),

		Ledger: ledger,
	}

	return newAvailabilityManager
}

func (a *AvailabilityManager) Solidify(params *DataFlowParams, next func(*DataFlowParams) error) error {
	if metadata.Solid() {
		return nil, true, false
	}

	if solid, inputs = a.allInputsAvailable(transaction); !solid {
		return nil, false, false
	}

	if !metadata.SetSolid(true) {
		return inputs, true, false
	}

	a.TransactionSolidEvent.Trigger(&TransactionSolidEvent{
		Inputs: inputs,
		TransactionStoredEvent: &TransactionStoredEvent{
			Transaction:         transaction,
			TransactionMetadata: metadata,
		},
	})

	return inputs, true, true
}

func (a *AvailabilityManager) allInputsAvailable(transaction utxo.Transaction) (allAvailable bool, outputs []utxo.Output) {
	inputs := transaction.Inputs()

	outputs = make([]utxo.Output, len(inputs))
	for i, input := range inputs {
		if !a.CachedOutput(a.vm.ResolveInput(input)).Consume(func(output utxo.Output) {
			outputs[i] = output
		}) {
			return false, nil
		}
	}

	return true, outputs
}
