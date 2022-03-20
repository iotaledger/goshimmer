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
	cachedInputs := objectstorage.CachedObjects[utxo.Output](generics.Map(generics.Map(params.Transaction.Inputs(), s.vm.ResolveInput), s.CachedOutput))
	defer cachedInputs.Release()

	if params.Inputs = cachedInputs.Unwrap(true); len(params.Inputs) != len(cachedInputs) {
		return nil
	}

	s.TransactionSolidEvent.Trigger(params.Transaction.ID())

	return next(params)
}
