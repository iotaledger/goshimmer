// Package cerrors provides sentinel error values for common errors that are likely to appear in multiple packages.
//
// Since wrapping errors using the new error handling API of Go creates anonymous errors it is not possible to wrap an
// existing sentinel error with another sentinel error without loosing the stack trace.
//
// Using shared sentinel errors enables them to be annotated and used across packages borders without the caller having
// to distinguish between similar errors types of different packages that would otherwise be identical.
package cerrors

import "errors"

var (
	// ErrBase58DecodeFailed is returned if a base58 encoded string can not be decoded.
	ErrBase58DecodeFailed = errors.New("failed to decode base58 encoded string")

	// ErrParseBytesFailed is returned if information can not be parsed from a sequence of bytes.
	ErrParseBytesFailed = errors.New("failed to parse bytes")
)
