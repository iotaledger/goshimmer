package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Validator struct {
	*Ledger
}

func NewValidator(ledger *Ledger) (new *Validator) {
	return &Validator{
		Ledger: ledger,
	}
}

func (v *Validator) checkOutputsCausallyRelatedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	cachedOutputsMetadata := v.CachedOutputsMetadata(params.InputIDs)
	defer cachedOutputsMetadata.Release()

	params.InputsMetadata = NewOutputsMetadata(cachedOutputsMetadata.Unwrap(true)...)
	if params.InputsMetadata.Size() != len(cachedOutputsMetadata) {
		return errors.Errorf("failed to retrieve the metadata of all inputs of %s: %w", params.Transaction.ID(), cerrors.ErrFatal)
	}

	if v.outputsCausallyRelated(params.InputsMetadata) {
		return errors.Errorf("%s is trying to spend causally related Outputs: %w", params.Transaction.ID(), ErrTransactionInvalid)
	}

	return next(params)
}

func (v *Validator) outputsCausallyRelated(outputsMetadata OutputsMetadata) (related bool) {
	spentOutputIDs := outputsMetadata.Filter((*OutputMetadata).Spent).IDs()
	if spentOutputIDs.Size() == 0 {
		return false
	}

	v.WalkConsumingTransactionMetadata(spentOutputIDs, func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		if !txMetadata.Booked() {
			return
		}

		for it := txMetadata.OutputIDs().Iterator(); it.HasNext(); {
			outputID := it.Next()

			if related = outputsMetadata.Has(outputID); related {
				walker.StopWalk()
				return
			}

			walker.Push(outputID)
		}
	})

	return related
}
