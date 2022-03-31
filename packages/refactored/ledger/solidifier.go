package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"
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
