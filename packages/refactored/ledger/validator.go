package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Validator struct {
	*Ledger
}

func (v *Validator) checkOutputsCausallyRelatedCommand(params *params, next dataflow.Next[*params]) (err error) {
	cachedOutputsMetadata := objectstorage.CachedObjects[*OutputMetadata](generics.Map(generics.Map(params.Inputs, utxo.Output.ID), v.CachedOutputMetadata))
	defer cachedOutputsMetadata.Release()

	params.InputsMetadata = generics.KeyBy[utxo.OutputID, *OutputMetadata](cachedOutputsMetadata.Unwrap(), (*OutputMetadata).ID)

	if v.outputsCausallyRelated(params.InputsMetadata) {
		return errors.Errorf("%s is trying to spend causally related Outputs: %w", params.Transaction.ID(), ErrTransactionInvalid)
	}

	return next(params)
}

func (v *Validator) outputsCausallyRelated(outputsMetadata map[utxo.OutputID]*OutputMetadata) (related bool) {
	spentOutputIDs := generics.Keys(generics.FilterByValue[utxo.OutputID, *OutputMetadata](outputsMetadata, (*OutputMetadata).Spent))
	if len(spentOutputIDs) == 0 {
		return false
	}

	v.WalkConsumingTransactionMetadata(spentOutputIDs, func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		for _, outputID := range txMetadata.OutputIDs() {
			if _, related = outputsMetadata[outputID]; related {
				walker.StopWalk()
				return
			}

			walker.Push(outputID)
		}
	})

	return related
}
