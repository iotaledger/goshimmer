package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type VM struct {
	*Ledger

	vm utxo.VM
}

func NewVM(ledger *Ledger, vm utxo.VM) (new *VM) {
	return &VM{
		Ledger: ledger,
		vm:     vm,
	}
}

func (v *VM) executeTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	utxoOutputs, err := v.vm.ExecuteTransaction(params.Transaction.Transaction, params.Inputs.UTXOOutputs())
	if err != nil {
		return errors.Errorf("failed to execute transaction with %s: %w", params.Transaction.ID(), ErrTransactionInvalid)
	}

	params.Outputs = NewOutputs(generics.Map(utxoOutputs, NewOutput)...)

	return next(params)
}

func (v *VM) resolveInputs(inputs []Input) (outputIDs OutputIDs) {
	return NewOutputIDs(generics.Map(inputs, v.vm.ResolveInput)...)
}
