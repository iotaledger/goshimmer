package utxo

// Output is the container for the data produced by executing a Transaction.
type Output interface {
	// ID returns the identifier of the Output.
	ID() (id OutputID)

	// SetID sets the identifier of the Output.
	//
	// Note: Since determining the identifier of an Output is an operation that requires at least a few hashing
	// operations, we allow the ID to be set externally.
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
