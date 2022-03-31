package utxo

// Transaction is the type that is used to describe instructions how to modify the ledger state.
type Transaction interface {
	// ID returns the identifier of the Transaction.
	ID() (id *TransactionID)

	// SetID sets the identifier of the Transaction.
	//
	// Note: Since determining the identifier of a Transaction is an operation that requires at least a few hashing
	// operations, we allow the ID to be set from the outside.
	//
	// This allows us to potentially skip the ID calculation in cases where the ID is known upfront (e.g. when loading
	// Transactions from the object storage).
	SetID(id *TransactionID)

	// Inputs returns the inputs of the Transaction.
	Inputs() (inputs []Input)

	// Bytes returns a serialized version of the Transaction.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the Transaction.
	String() (humanReadable string)
}
