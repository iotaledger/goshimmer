package ledger

type TransactionStoredEvent struct {
	*DataFlowParams
}

type TransactionSolidEvent struct {
	*DataFlowParams
}

type TransactionProcessedEvent struct {
	*DataFlowParams
}
