package ledgerstate

// ConfirmationOracle answers questions about entities' confirmation.
type ConfirmationOracle interface {
	IsTransactionConfirmed(transactionID TransactionID) bool
	IsTransactionRejected(transactionID TransactionID) bool
	IsBranchConfirmed(branchID BranchID) bool
	IsBranchRejected(branchID BranchID) bool
}
