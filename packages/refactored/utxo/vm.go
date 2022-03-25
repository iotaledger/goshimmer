package utxo

// VM is a generic interface for UTXO-based VMs that are compatible with the IOTA 2.0 consensus.
type VM interface {
	// ExecuteTransaction executes the Transaction and determines the Outputs from the given Inputs. It returns an error
	// if the execution fails.
	ExecuteTransaction(transaction Transaction, inputs []Output, executionLimit ...uint64) (outputs []Output, err error)

	// ParseTransaction unmarshals a Transaction from the given sequence of bytes.
	ParseTransaction([]byte) (transaction Transaction, err error)

	// ParseOutput unmarshals an Output from the given sequence of bytes.
	ParseOutput([]byte) (output Output, err error)

	// ResolveInput translates the Input into an OutputID.
	ResolveInput(input Input) (outputID OutputID)
}
