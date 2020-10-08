package payload

import (
	"errors"
)

var (
	// ErrParseBytesFailed is returned if information can not be parsed from a sequence of bytes.
	ErrParseBytesFailed = errors.New("failed to parse bytes")
)
