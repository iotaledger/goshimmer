package utxo

// OutputID is the type that is used for identifiers of Outputs.
type OutputID interface {
	// MapKey returns an array version of the OutputID which can be used as a key in maps.
	MapKey() [32]byte

	// Bytes returns a serialized version of the OutputID.
	Bytes() (serialized []byte)

	// String returns a human-readable version of the OutputID.
	String() (humanReadable string)
}
