package transport

import "errors"

var (
	ErrClosed         = errors.New("socket closed")
	ErrUnsupported    = errors.New("operation not supported")
	ErrInvalidAddress = errors.New("invalid address")
)
