package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

type validator struct {
	*Ledger
}

func newValidator(ledger *Ledger) (new *validator) {
	return &validator{
		Ledger: ledger,
	}
}

func (v *validator) checkSolidityCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.InputIDs.IsEmpty() {
		params.InputIDs = v.utils.resolveInputs(params.Transaction.Inputs())
	}

	cachedInputs := v.Storage.CachedOutputs(params.InputIDs)
	defer cachedInputs.Release()
	if params.Inputs = NewOutputs(cachedInputs.Unwrap(true)...); params.Inputs.Size() != len(cachedInputs) {
		return errors.Errorf("not all outputs of %s available: %w", params.Transaction.ID(), ErrTransactionUnsolid)
	}

	return next(params)
}

func (v *validator) checkOutputsCausallyRelatedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	cachedOutputsMetadata := v.Storage.CachedOutputsMetadata(params.InputIDs)
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

func (v *validator) checkTransactionExecutionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	utxoOutputs, err := v.Options.VM.ExecuteTransaction(params.Transaction.Transaction, params.Inputs.UTXOOutputs())
	if err != nil {
		return errors.Errorf("failed to execute transaction with %s: %w", params.Transaction.ID(), ErrTransactionInvalid)
	}

	params.Outputs = NewOutputs(lo.Map(utxoOutputs, NewOutput)...)

	return next(params)
}

func (v *validator) outputsCausallyRelated(outputsMetadata OutputsMetadata) (related bool) {
	spentOutputIDs := outputsMetadata.Filter((*OutputMetadata).Spent).IDs()
	if spentOutputIDs.Size() == 0 {
		return false
	}

	v.utils.WalkConsumingTransactionMetadata(spentOutputIDs, func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
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
