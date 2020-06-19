package tangle

import "errors"

var (
	// ErrFatal represents an error that is not "expected".
	ErrFatal = errors.New("fatal error")

	// ErrTransactionInvalid represents an error type that is triggered when an invalid transaction is detected.
	ErrTransactionInvalid = errors.New("transaction invalid")

	// ErrPayloadInvalid represents an error type that is triggered when an invalid payload is detected.
	ErrPayloadInvalid = errors.New("payload invalid")
)
