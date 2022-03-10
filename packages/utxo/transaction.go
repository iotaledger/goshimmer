package utxo

type Transaction interface {
	ID() TransactionID

	Inputs() []Input

	Bytes() []byte
}
