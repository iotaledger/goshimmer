package devnetvm

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm"
)

type VM struct{}

func (d *VM) ParseTransaction(transactionBytes []byte) (transaction utxo.Transaction, err error) {
	tx := new(Transaction)
	err = tx.FromBytes(transactionBytes)
	return tx, err
}

func (d *VM) ParseOutput(outputBytes []byte) (output utxo.Output, err error) {
	if output, err = OutputFromBytes(outputBytes); err != nil {
		err = errors.Wrap(err, "failed to parse Output")
	}

	return output, err
}

func (d *VM) ResolveInput(input utxo.Input) (outputID utxo.OutputID) {
	return input.(*UTXOInput).ReferencedOutputID()
}

func (d *VM) ExecuteTransaction(transaction utxo.Transaction, inputs *utxo.Outputs, _ ...uint64) (outputs []utxo.Output, err error) {
	typedOutputs, err := d.executeTransaction(transaction.(*Transaction), OutputsFromUTXOOutputs(inputs))
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute transaction")
	}

	return typedOutputs.UTXOOutputs(), nil
}

func (d *VM) executeTransaction(transaction *Transaction, inputs Outputs) (outputs Outputs, err error) {
	if !TransactionBalancesValid(inputs, transaction.Essence().Outputs()) {
		return nil, errors.WithMessagef(ErrTransactionInvalid, "sum of consumed and spent balances is not 0")
	}
	if !UnlockBlocksValid(inputs, transaction) {
		return nil, errors.WithMessagef(ErrTransactionInvalid, "spending of referenced consumedOutputs is not authorized")
	}
	if !AliasInitialStateValid(inputs, transaction) {
		return nil, errors.WithMessagef(ErrTransactionInvalid, "initial state of created alias output is invalid")
	}

	outputs = make(Outputs, 0, len(transaction.Essence().Outputs()))
	for i, output := range transaction.Essence().Outputs() {
		output.SetID(utxo.NewOutputID(transaction.ID(), uint16(i)))
		updatedOutput := output.UpdateMintingColor()
		outputs = append(outputs, updatedOutput)
	}

	return outputs, nil
}

var _ vm.VM = new(VM)
