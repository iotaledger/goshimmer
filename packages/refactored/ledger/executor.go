package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
)

type Executor struct {
	*Ledger
}

func NewExecutor(ledger *Ledger) (new *Executor) {
	return &Executor{
		Ledger: ledger,
	}
}

func (e *Executor) executeTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	utxoOutputs, err := e.vm.ExecuteTransaction(params.Transaction.Transaction, generics.Map(params.Inputs, (*Output).utxoOutput))
	if err != nil {
		return errors.Errorf("failed to execute transaction with %s: %w", params.Transaction.ID(), ErrTransactionInvalid)
	}

	params.Outputs = generics.Map(utxoOutputs, NewOutput)

	return next(params)
}
