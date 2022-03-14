package txvm

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"

	utxo2 "github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type VM struct{}

func (d *VM) ParseTransaction(transactionBytes []byte) (transaction utxo2.Transaction, err error) {
	return new(Transaction).FromMarshalUtil(marshalutil.New(transactionBytes))
}

func (d *VM) ParseOutput(outputBytes []byte) (output utxo2.Output, err error) {
	return OutputFromMarshalUtil(marshalutil.New(outputBytes))
}

func (d *VM) ResolveInput(input utxo2.Input) (outputID utxo2.OutputID) {
	return input.(*UTXOInput).ReferencedOutputID()
}

func (d *VM) ExecuteTransaction(transaction utxo2.Transaction, inputs []utxo2.Output, _ ...uint64) (outputs []utxo2.Output, err error) {
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

var _ utxo2.VM = new(VM)
