package ledgerstate

import (
	"errors"
)

var (
	// ErrBase58DecodeFailed is returned if a base58 encoded string can not be decoded.
	ErrBase58DecodeFailed = errors.New("failed to decode base58 encoded string")

	// ErrParseBytesFailed is returned if information can not be parsed from a sequence of bytes.
	ErrParseBytesFailed = errors.New("failed to parse bytes")

	// ErrTransactionInvalid is returned if a transaction or any of its building blocks is considered to be invalid.
	ErrTransactionInvalid = errors.New("transaction invalid")

	// ErrInvalidArgument is returned if a function is called with an argument that is not allowed.
	ErrInvalidArgument = errors.New("invalid argument")
)
