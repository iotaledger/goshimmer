package tangle

import "errors"

var (
	// ErrNotSynced is triggered when somebody tries to issue a Payload before the Tangle is fully synced.
	ErrNotSynced = errors.New("tangle not synced")
)
