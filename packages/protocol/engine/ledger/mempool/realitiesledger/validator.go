package realitiesledger

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/cerrors"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/core/dataflow"
	"github.com/iotaledger/hive.go/ds/walker"
)

// validator is a RealitiesLedger component that bundles the API that is used to check the validity of a Transaction.
type validator struct {
	ledger *RealitiesLedger
}

// newValidator returns a new validator instance for the given RealitiesLedger.
func newValidator(ledger *RealitiesLedger) *validator {
	return &validator{
		ledger: ledger,
	}
}

// checkSolidityCommand is a ChainedCommand that aborts the DataFlow if the Transaction is not solid.
func (v *validator) checkSolidityCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.InputIDs.IsEmpty() {
		params.InputIDs = v.ledger.utils.ResolveInputs(params.Transaction.Inputs())
	}

	cachedInputs := v.ledger.storage.CachedOutputs(params.InputIDs)
	defer cachedInputs.Release()
	if params.Inputs = utxo.NewOutputs(cachedInputs.Unwrap(true)...); params.Inputs.Size() != len(cachedInputs) {
		return errors.WithMessagef(mempool.ErrTransactionUnsolid, "not all outputs of %s available", params.Transaction.ID())
	}

	return next(params)
}

// checkOutputsCausallyRelatedCommand is a ChainedCommand that aborts the DataFlow if the spent Outputs reference each
// other.
func (v *validator) checkOutputsCausallyRelatedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	cachedOutputsMetadata := v.ledger.storage.CachedOutputsMetadata(params.InputIDs)
	defer cachedOutputsMetadata.Release()

	params.InputsMetadata = mempool.NewOutputsMetadata(cachedOutputsMetadata.Unwrap(true)...)
	if params.InputsMetadata.Size() != len(cachedOutputsMetadata) {
		return errors.WithMessagef(cerrors.ErrFatal, "failed to retrieve the metadata of all inputs of %s", params.Transaction.ID())
	}

	if v.outputsCausallyRelated(params.InputsMetadata) {
		return errors.WithMessagef(mempool.ErrTransactionInvalid, "%s is trying to spend causally related Outputs", params.Transaction.ID())
	}

	return next(params)
}

// checkTransactionExecutionCommand is a ChainedCommand that aborts the DataFlow if the Transaction could not be
// executed (is invalid).
func (v *validator) checkTransactionExecutionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	utxoOutputs, err := v.ledger.optsVM.ExecuteTransaction(params.Transaction, params.Inputs)
	if err != nil {
		return errors.WithMessagef(mempool.ErrTransactionInvalid, "failed to execute transaction with %s: %s", params.Transaction.ID(), err.Error())
	}

	params.Outputs = utxo.NewOutputs(utxoOutputs...)

	return next(params)
}

// outputsCausallyRelated returns true if the Outputs denoted by the given OutputsMetadata reference each other.
func (v *validator) outputsCausallyRelated(outputsMetadata *mempool.OutputsMetadata) (related bool) {
	spentOutputIDs := outputsMetadata.Filter((*mempool.OutputMetadata).IsSpent).IDs()
	if spentOutputIDs.Size() == 0 {
		return false
	}

	v.ledger.utils.WalkConsumingTransactionMetadata(spentOutputIDs, func(txMetadata *mempool.TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
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
