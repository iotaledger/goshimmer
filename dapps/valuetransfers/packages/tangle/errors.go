package tangle

import "errors"

var (
	ErrFatal              = errors.New("fatal error")
	ErrTransactionInvalid = errors.New("transaction invalid")
	ErrPayloadInvalid     = errors.New("payload invalid")
)
