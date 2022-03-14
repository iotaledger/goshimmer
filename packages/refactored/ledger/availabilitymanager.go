package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type AvailabilityManager struct {
	*Ledger
}

func NewAvailabilityManager(ledger *Ledger) (newAvailabilityManager *AvailabilityManager) {
	newAvailabilityManager = new(AvailabilityManager)
	newAvailabilityManager.Ledger = ledger

	return newAvailabilityManager
}

func (a *AvailabilityManager) Setup() {
	a.TransactionStored.Attach(events.NewClosure(a.onTransactionStored))
}

func (a *AvailabilityManager) onTransactionStored(transaction utxo.Transaction) {
	if _, err := a.CheckAvailability(transaction); err != nil {
		a.Error.Trigger(errors.Errorf("failed to check availability of transaction with %s: %w", transaction.ID(), err))
	}
}

func (a *AvailabilityManager) CheckAvailability(transaction utxo.Transaction) (available bool, err error) {
	inputs, allAvailable := a.allInputsAvailable(transaction)
	if !allAvailable {
		return false, nil
	}

	return true, nil
}

func (a *AvailabilityManager) allInputsAvailable(transaction utxo.Transaction) (allAvailable bool, outputs []utxo.Output) {
	inputs := transaction.Inputs()

	outputs = make([]utxo.Output, len(inputs))
	for i, input := range inputs {
		if !a.CachedOutput(a.ResolveInput(input)).Consume(func(output utxo.Output) {
			outputs[i] = output
		}) {
			return false, nil
		}
	}

	return true, outputs
}
