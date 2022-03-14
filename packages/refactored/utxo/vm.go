package utxo

type VM interface {
	ParseTransaction(transactionBytes []byte) (transaction Transaction, err error)

	ParseOutput(outputBytes []byte) (output Output, err error)

	ResolveInput(input Input) (outputID OutputID)

	ExecuteTransaction(transaction Transaction, inputs []Output, executionLimit ...uint64) (outputs []Output, err error)
}
