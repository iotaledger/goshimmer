package gossip

import "github.com/cockroachdb/errors"

var (
	// ErrNotRunning is returned when a neighbor is added to a stopped or not yet started gossip manager.
	ErrNotRunning = errors.New("manager not running")
)
