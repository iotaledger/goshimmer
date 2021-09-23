package tangle

import "github.com/cockroachdb/errors"

var (
	// ErrNotSynced is triggered when somebody tries to issue a Payload before the Tangle is fully synced.
	ErrNotSynced = errors.New("tangle not synced")
	// ErrInvalidInputs is returned when one or more inputs are rejected or non-monotonically liked.
	ErrInvalidInputs = errors.New("one or more inputs are rejected or non-monotonically liked")
	// ErrParentsInvalid is returned when one or more parents of a message is invalid.
	ErrParentsInvalid = errors.New("one or more parents is invalid")
)
