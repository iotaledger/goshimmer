package utxo

// Transaction is the type that is used to describe instructions how to modify the ledger state.
type Transaction interface {
	// ID returns the identifier of the Transaction.
	ID() (id TransactionID)

	// Inputs returns the inputs of the Transaction.
	Inputs() (inputs []Input)

	// Bytes returns a serialized version of the Transaction.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the Transaction.
	String() (humanReadable string)
}
