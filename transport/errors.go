package transport

import "errors"

var (
	errClosed         = errors.New("socket closed")
	errUnsupported    = errors.New("operation not supported")
	errInvalidAddress = errors.New("invalid address")
)
