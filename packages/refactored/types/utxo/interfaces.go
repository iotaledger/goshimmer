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

// Transaction is the type that is used to describe instructions how to modify the ledger state.
type Transaction interface {
	// ID returns the identifier of the Transaction.
	ID() (id TransactionID)

	// SetID sets the identifier of the Transaction.
	//
	// Note: Since determining the identifier of a Transaction is an operation that requires at least a few hashing
	// operations, we allow the ID to be set from the outside.
	//
	// This allows us to potentially skip the ID calculation in cases where the ID is known upfront (e.g. when loading
	// Transactions from the object storage).
	SetID(id TransactionID)

	// Inputs returns the inputs of the Transaction.
	Inputs() (inputs []Input)

	// Bytes returns a serialized version of the Transaction.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the Transaction.
	String() (humanReadable string)
}

// Input is an entity that allows to encode information about which Outputs are supposed to be used by a Transaction.
type Input interface {
	// Bytes returns a serialized version of the Input.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the Input.
	String() (humanReadable string)
}

// Output is the container for the data produced by executing a Transaction.
type Output interface {
	// ID returns the identifier of the Output.
	ID() (id OutputID)

	// SetID sets the identifier of the Output.
	//
	// Note: Since determining the identifier of an Output is an operation that requires at least a few hashing
	// operations, we allow the ID to be set from the outside.
	//
	// This allows us to potentially skip the ID calculation in cases where the ID is known upfront (e.g. when loading
	// Outputs from the object storage).
	SetID(id OutputID)

	// TransactionID returns the identifier of the Transaction that created this Output.
	TransactionID() (txID TransactionID)

	// Index returns the unique Index of the Output in respect to its TransactionID.
	Index() (index uint16)

	// Bytes returns a serialized version of the Output.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the Output.
	String() (humanReadable string)
}
