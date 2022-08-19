package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/generics/dataflow"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

// validator is a Ledger component that bundles the API that is used to check the validity of a Transaction.
type validator struct {
	ledger *Ledger
}

// newValidator returns a new validator instance for the given Ledger.
func newValidator(ledger *Ledger) (new *validator) {
	return &validator{
		ledger: ledger,
	}
}

// checkSolidityCommand is a ChainedCommand that aborts the DataFlow if the Transaction is not solid.
func (v *validator) checkSolidityCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.InputIDs.IsEmpty() {
		params.InputIDs = v.ledger.Utils.ResolveInputs(params.Transaction.Inputs())
	}

	cachedInputs := v.ledger.Storage.CachedOutputs(params.InputIDs)
	defer cachedInputs.Release()
	if params.Inputs = utxo.NewOutputs(cachedInputs.Unwrap(true)...); params.Inputs.Size() != len(cachedInputs) {
		return errors.Errorf("not all outputs of %s available: %w", params.Transaction.ID(), ErrTransactionUnsolid)
	}

	return next(params)
}

// checkOutputsCausallyRelatedCommand is a ChainedCommand that aborts the DataFlow if the spent Outputs reference each
// other.
func (v *validator) checkOutputsCausallyRelatedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	cachedOutputsMetadata := v.ledger.Storage.CachedOutputsMetadata(params.InputIDs)
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

// checkTransactionExecutionCommand is a ChainedCommand that aborts the DataFlow if the Transaction could not be
// executed (is invalid).
func (v *validator) checkTransactionExecutionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	utxoOutputs, err := v.ledger.options.vm.ExecuteTransaction(params.Transaction, params.Inputs)
	if err != nil {
		return errors.Errorf("failed to execute transaction with %s: %w: %w", params.Transaction.ID(), ErrTransactionInvalid, err)
	}

	params.Outputs = utxo.NewOutputs(utxoOutputs...)

	return next(params)
}

// outputsCausallyRelated returns true if the Outputs denoted by the given OutputsMetadata reference each other.
func (v *validator) outputsCausallyRelated(outputsMetadata *OutputsMetadata) (related bool) {
	spentOutputIDs := outputsMetadata.Filter((*OutputMetadata).IsSpent).IDs()
	if spentOutputIDs.Size() == 0 {
		return false
	}

	v.ledger.Utils.WalkConsumingTransactionMetadata(spentOutputIDs, func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		if !txMetadata.IsBooked() {
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
