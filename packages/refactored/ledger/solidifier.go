package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Solidifier struct {
	*Ledger
}

func NewSolidifier(ledger *Ledger) (new *Solidifier) {
	return &Solidifier{
		Ledger: ledger,
	}
}

func (s *Solidifier) checkSolidityCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.InputIDs.IsEmpty() {
		params.InputIDs = s.resolveInputs(params.Transaction.Inputs())
	}

	cachedInputs := s.CachedOutputs(params.InputIDs)
	defer cachedInputs.Release()
	if params.Inputs = NewOutputs(cachedInputs.Unwrap(true)...); params.Inputs.Size() != len(cachedInputs) {
		return errors.Errorf("not all outputs of %s available: %w", params.Transaction.ID(), ErrTransactionUnsolid)
	}

	return next(params)
}

func (s *Solidifier) initConsumers(outputIDs OutputIDs, txID TransactionID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	_ = outputIDs.ForEach(func(outputID utxo.OutputID) (err error) {
		cachedConsumers = append(cachedConsumers, s.CachedConsumer(outputID, txID, NewConsumer))
		return nil
	})

	return cachedConsumers
}
