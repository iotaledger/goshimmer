package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type DataFlow struct {
	*Ledger
}

func (d *DataFlow) lockTransaction(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) error {
	d.Lock(params.Transaction, false)

	return next(params)
}

func (d *DataFlow) unlockTransaction(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) error {
	d.Unlock(params.Transaction, false)

	return next(params)
}

func (d *DataFlow) CheckSolidityCommand(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) (err error) {
	if params.Inputs = d.CheckSolidity(params.Transaction, params.TransactionMetadata); params.Inputs == nil {
		return nil
	}

	return next(params)
}

func (d *DataFlow) SolidifyTransaction(tx utxo.Transaction, meta *TransactionMetadata) (success bool, consumers []utxo.TransactionID, err error) {
	err = dataflow.New[*DataFlowParams](
		d.lockTransaction,
		d.CheckSolidityCommand,
		d.ValidatePastCone,
		d.ExecuteTransaction,
		d.BookTransaction,
	).WithSuccessCallback(func(params *DataFlowParams) {
		success = true
		// TODO: fill consumers from outputs
	}).WithTerminationCallback(func(params *DataFlowParams) {
		d.Unlock(params.Transaction, false)
	}).Run(&DataFlowParams{
		Transaction:         tx,
		TransactionMetadata: meta,
	})

	if err != nil {
		// TODO: mark Transaction as invalid and trigger invalid event
		// eventually trigger generic errors if its not due to tx invalidity
		return false, nil, err
	}

	return success, consumers, nil
}

type Command dataflow.ChainedCommand[*DataFlowParams]
