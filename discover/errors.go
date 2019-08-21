package discover

import "github.com/pkg/errors"

var (
	errTimeout = errors.New("RPC timeout")
	errClosed  = errors.New("socket closed")
)
