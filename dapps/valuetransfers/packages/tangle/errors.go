package tangle

import "errors"

var (
	// ErrFatal represents an error that is not "expected".
	ErrFatal = errors.New("fatal error")

	// ErrTransactionInvalid represents an error type that is triggered when an invalid transaction is detected.
	ErrTransactionInvalid = errors.New("transaction invalid")

	// ErrPayloadInvalid represents an error type that is triggered when an invalid payload is detected.
	ErrPayloadInvalid = errors.New("payload invalid")

	// ErrDoubleSpendForbidden represents an error that is triggered when a user tries to issue a double spend.
	ErrDoubleSpendForbidden = errors.New("it is not allowed to issue a double spend")

	// ErrTransactionDoesNotSpendAllFunds is returned if a transaction does not spend all of its inputs.
	ErrTransactionDoesNotSpendAllFunds = errors.New("transaction does not spend all funds from inputs")
	// ErrInvalidTransactionSignature is returned if the signature of a transaction is invalid.
	ErrInvalidTransactionSignature = errors.New("missing or invalid transaction signature")
	// ErrMaxTransactionInputCountExceeded is returned if the max number of inputs of the transaction is exceeded.
	ErrMaxTransactionInputCountExceeded = errors.New("maximum transaction input count exceeded")
)
