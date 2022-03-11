package utxo

type Output interface {
	ID() (id OutputID)

	Bytes() (serializedOutput []byte)

	String() (humanReadableOutput string)
}
