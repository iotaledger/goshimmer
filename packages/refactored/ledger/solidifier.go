package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Solidifier struct {
	*Ledger
}

func NewSolidifier(ledger *Ledger) (newAvailabilityManager *Solidifier) {
	return &Solidifier{
		Ledger: ledger,
	}
}

func (s *Solidifier) checkSolidityCommand(params *params, next dataflow.Next[*params]) (err error) {
	params.InputsIDs = s.outputIDsFromInputs(params.Transaction.Inputs())

	cachedInputs := objectstorage.CachedObjects[*Output](generics.Map(params.InputsIDs, s.CachedOutput))
	defer cachedInputs.Release()

	if params.Inputs = cachedInputs.Unwrap(true); len(params.Inputs) != len(cachedInputs) {
		return nil
	}

	cachedConsumers := objectstorage.CachedObjects[*Consumer](generics.Map(params.InputsIDs, generics.Bind(params.Transaction.ID(), s.CachedConsumer)))
	defer cachedConsumers.Release()

	params.Consumers = cachedConsumers.Unwrap()

	s.TransactionSolidEvent.Trigger(params.Transaction.ID())

	return next(params)
}

func (s *Solidifier) outputIDsFromInputs(inputs []utxo.Input) (outputIDs []utxo.OutputID) {
	return generics.Map(inputs, s.vm.ResolveInput)
}
