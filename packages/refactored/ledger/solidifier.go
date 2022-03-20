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

func (s *Solidifier) checkSolidity(transaction utxo.Transaction, metadata *TransactionMetadata) (success bool, inputs []utxo.Output) {
	if metadata.Solid() {
		return false, nil
	}

	cachedInputs := objectstorage.CachedObjects[utxo.Output](generics.Map(generics.Map(transaction.Inputs(), s.vm.ResolveInput), s.CachedOutput))
	defer cachedInputs.Release()

	inputs = cachedInputs.Unwrap(true)
	if len(inputs) != len(cachedInputs) {
		return false, nil
	}

	if !metadata.SetSolid(true) {
		return false, nil
	}

	s.TransactionSolidEvent.Trigger(transaction.ID())

	return true, inputs
}

func (s *Solidifier) checkSolidityCommand(params *params, next dataflow.Next[*params]) (err error) {
	success, inputs := s.checkSolidity(params.Transaction, params.TransactionMetadata)
	if !success {
		return nil
	}

	params.Inputs = inputs

	return next(params)
}
