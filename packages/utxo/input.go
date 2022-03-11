package utxo

type Input interface {
	String() (humanReadableInput string)
}
