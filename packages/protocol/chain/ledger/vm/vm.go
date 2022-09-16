package vm

import (
	utxo2 "github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/utxo"
)

// VM is a generic interface for UTXO-based VMs.
type VM interface {
	// ExecuteTransaction executes the Transaction and determines the Outputs from the given Inputs. It returns an error
	// if the execution fails.
	ExecuteTransaction(transaction utxo2.Transaction, inputs *utxo2.Outputs, gasLimit ...uint64) (outputs []utxo2.Output, err error)

	// ParseTransaction un-serializes a Transaction from the given sequence of bytes.
	ParseTransaction([]byte) (transaction utxo2.Transaction, err error)

	// ParseOutput un-serializes an Output from the given sequence of bytes.
	ParseOutput([]byte) (output utxo2.Output, err error)

	// ResolveInput translates the Input into an OutputID.
	ResolveInput(input utxo2.Input) (outputID utxo2.OutputID)
}
