package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"
)

type Executor struct {
	*Ledger
}

func (e *Executor) executeTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	if params.Outputs, err = e.vm.ExecuteTransaction(params.Transaction, params.Inputs); err != nil {
		return errors.Errorf("failed to execute transaction with %s: %w", params.Transaction.ID(), ErrTransactionInvalid)
	}

	return next(params)
}
