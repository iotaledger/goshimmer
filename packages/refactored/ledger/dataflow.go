package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
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

func (d *DataFlow) LockedCommand(command Command) (lockedCommand Command) {
	return dataflow.New[*DataFlowParams](
		d.lockTransaction,
		command,
	).WithTerminationCallback(func(params *DataFlowParams) {
		d.Unlock(params.Transaction, false)
	}).ChainedCommand
}

func (d *DataFlow) CheckSolidityCommand(params *DataFlowParams, next dataflow.Next[*DataFlowParams]) (err error) {
	if params.Inputs = d.CheckSolidity(params.Transaction, params.TransactionMetadata); params.Inputs == nil {
		return nil
	}

	return next(params)
}

func (d *DataFlow) SolidifyTransactionCommand() (solidificationCommand Command) {
	return d.LockedCommand(
		dataflow.New[*DataFlowParams](
			d.CheckSolidityCommand,
			d.ValidatePastCone,
			d.ExecuteTransaction,
			d.BookTransaction,
		).WithErrorCallback(func(err error, params *DataFlowParams) {
			// mark Transaction as invalid and trigger invalid event

			// eventually trigger generic errors if its not due to tx invalidity
		}),
	)
}

type Command dataflow.ChainedCommand[*DataFlowParams]
