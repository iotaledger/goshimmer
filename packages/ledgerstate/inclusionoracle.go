package ledgerstate

// ConfirmationOracle answers questions about entities' confirmation.
type ConfirmationOracle interface {
	IsTransactionConfirmed(transactionID TransactionID) bool
	IsTransactionRejected(transactionID TransactionID) bool
}
