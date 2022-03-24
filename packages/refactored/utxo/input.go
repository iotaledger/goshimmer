package utxo

// Input is an entity that allows to encode information about which Outputs are supposed to be used by a Transaction.
type Input interface {
	// Bytes returns a serialized version of the Input.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the Input.
	String() (humanReadable string)
}
