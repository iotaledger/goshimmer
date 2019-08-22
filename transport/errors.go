package transport

import "errors"

var (
	errClosed = errors.New("socket closed")
	errPeer   = errors.New("could not determine peer")
)
