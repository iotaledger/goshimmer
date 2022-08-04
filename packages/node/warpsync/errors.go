package warpsync

import "github.com/cockroachdb/errors"

var (
	// ErrNotRunning is returned when the Warpsync manager is not running.
	ErrNotRunning = errors.New("manager not running")
)
