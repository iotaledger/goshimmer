package utxo

// VM is a generic interface for UTXO-based VMs that are compatible with the IOTA 2.0 consensus.
type VM interface {
	// ParseTransaction parses the given bytes and returns the corresponding Transaction (or an error if the parsing
	// failed).
	ParseTransaction(transactionBytes []byte) (transaction Transaction, err error)

	// ParseOutput parses the given bytes and returns the corresponding Output (or an error if the parsing failed).
	ParseOutput(outputBytes []byte) (output Output, err error)

	// ResolveInput translates the Input into an OutputID.
	ResolveInput(input Input) (outputID OutputID)

	// ExecuteTransaction executes the Transaction and determines the Outputs from the given Inputs. It returns an error
	// if the execution fails.
	ExecuteTransaction(transaction Transaction, inputs []Output, executionLimit ...uint64) (outputs []Output, err error)
}
