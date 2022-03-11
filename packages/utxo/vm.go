package utxo

type VM interface {
	ExecuteTransaction(transaction Transaction, inputs []Output, executionLimit ...uint64) (outputs []Output, err error)
}
