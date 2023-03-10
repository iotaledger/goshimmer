package vm

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
)

// VM is a generic interface for UTXO-based VMs.
type VM interface {
	// ExecuteTransaction executes the Transaction and determines the Outputs from the given Inputs. It returns an error
	// if the execution fails.
	ExecuteTransaction(transaction utxo.Transaction, inputs *utxo.Outputs, gasLimit ...uint64) (outputs []utxo.Output, err error)

	// ParseTransaction un-serializes a Transaction from the given sequence of bytes.
	ParseTransaction([]byte) (transaction utxo.Transaction, err error)

	// ParseOutput un-serializes an Output from the given sequence of bytes.
	ParseOutput([]byte) (output utxo.Output, err error)

	// ResolveInput translates the Input into an OutputID.
	ResolveInput(input utxo.Input) (outputID utxo.OutputID)
}
