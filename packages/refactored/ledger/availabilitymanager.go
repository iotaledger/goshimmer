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

func (a *AvailabilityManager) CheckSolidity(transaction utxo.Transaction, metadata *TransactionMetadata) (inputs []utxo.Output) {
	if metadata.Solid() {
		return nil
	}

	solid, inputs := a.allInputsAvailable(transaction)
	if !solid {
		return nil
	}

	if !metadata.SetSolid(true) {
		return nil
	}

	a.TransactionSolidEvent.Trigger(&TransactionSolidEvent{
		Inputs: inputs,
		TransactionStoredEvent: &TransactionStoredEvent{
			Transaction:         transaction,
			TransactionMetadata: metadata,
		},
	})

	return inputs
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
