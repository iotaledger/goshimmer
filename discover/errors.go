package discover

import "github.com/pkg/errors"

var (
	errTimeout   = errors.New("RPC timeout")
	errClosed    = errors.New("socket closed")
	errNoMessage = errors.New("packet does not contain a message")
)
