package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Executor struct {
	*Ledger

	vm utxo.VM
}

func (e *Executor) executeTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	utxoOutputs, err := e.vm.ExecuteTransaction(params.Transaction, generics.Map(params.Inputs, (*Output).utxoOutput))
	if err != nil {
		return errors.Errorf("failed to execute transaction with %s: %w", params.Transaction.ID(), ErrTransactionInvalid)
	}

	params.Outputs = generics.Map(utxoOutputs, NewOutput)

	return next(params)
}

func (e *Executor) utxoOutputToOutput(output utxo.Output) *Output {
	return &Output{Output: output}
}
