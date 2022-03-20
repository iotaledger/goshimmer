package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/refactored/g"
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
	params.InputsIDs = g.Map(params.Transaction.Inputs(), s.vm.ResolveInput)

	cachedConsumers := objectstorage.CachedObjects[*Consumer](g.Map(params.InputsIDs, g.Bind(s.CachedConsumer, params.Transaction.ID())))
	defer cachedConsumers.Release()
	cachedInputs := objectstorage.CachedObjects[utxo.Output](g.Map(params.InputsIDs, s.CachedOutput))
	defer cachedInputs.Release()

	if params.Inputs = cachedInputs.Unwrap(true); len(params.Inputs) != len(cachedInputs) {
		return nil
	}
	params.Consumers = cachedConsumers.Unwrap()

	s.TransactionSolidEvent.Trigger(params.Transaction.ID())

	return next(params)
}
