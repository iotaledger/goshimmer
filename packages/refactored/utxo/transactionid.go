package utxo

// TransactionID is the type that is used for identifiers of Transactions.
type TransactionID interface {
	// MapKey returns an array version of the TransactionID which can be used as a key in maps.
	MapKey() [32]byte

	// Bytes returns a serialized version of the TransactionID.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the TransactionID.
	String() (humanReadable string)
}
