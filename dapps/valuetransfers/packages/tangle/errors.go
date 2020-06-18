package tangle

import "errors"

var (
	// ErrFatal represents an error that is not "expected".
	ErrFatal = errors.New("fatal error")

	// ErrTransactionInvalid represents an error that is triggered when an invalid transaction is issued.
	ErrTransactionInvalid = errors.New("transaction invalid")

	//
	ErrPayloadInvalid = errors.New("payload invalid")
)
