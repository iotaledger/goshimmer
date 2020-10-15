// Package cerrors provides sentinel error values for "common errors" that are likely to appear in multiple packages.
package cerrors

import "errors"

var (
	// Base58DecodeFailed is returned if a base58 encoded string can not be decoded.
	Base58DecodeFailed = errors.New("failed to decode base58 encoded string")

	// ParseBytesFailed is returned if information can not be parsed from a sequence of bytes.
	ParseBytesFailed = errors.New("failed to parse bytes")
)
