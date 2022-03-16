package txvm

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type VM struct{}

func (d *VM) ParseTransaction(transactionBytes []byte) (transaction utxo.Transaction, err error) {
	return new(Transaction).FromMarshalUtil(marshalutil.New(transactionBytes))
}

func (d *VM) ParseOutput(outputBytes []byte) (output utxo.Output, err error) {
	return OutputFromMarshalUtil(marshalutil.New(outputBytes))
}

func (d *VM) ResolveInput(input utxo.Input) (outputID utxo.OutputID) {
	return input.(*UTXOInput).ReferencedOutputID()
}

func (d *VM) ExecuteTransaction(transaction utxo.Transaction, inputs []utxo.Output, _ ...uint64) (outputs []utxo.Output, err error) {
	typedOutputs, err := d.executeTransaction(transaction.(*Transaction), OutputsFromUTXOOutputs(inputs))
	if err != nil {
		return nil, errors.Errorf("failed to execute transaction: %w", err)
	}

	return typedOutputs.UTXOOutputs(), nil
}

func (d *VM) executeTransaction(transaction *Transaction, inputs Outputs) (outputs Outputs, err error) {
	if !TransactionBalancesValid(inputs, transaction.Essence().Outputs()) {
		return nil, errors.Errorf("sum of consumed and spent balances is not 0: %w", ErrTransactionInvalid)
	}
	if !UnlockBlocksValid(inputs, transaction) {
		return nil, errors.Errorf("spending of referenced consumedOutputs is not authorized: %w", ErrTransactionInvalid)
	}
	if !AliasInitialStateValid(inputs, transaction) {
		return nil, errors.Errorf("initial state of created alias output is invalid: %w", ErrTransactionInvalid)
	}

	return transaction.Essence().Outputs(), nil
}

var _ utxo.VM = new(VM)
