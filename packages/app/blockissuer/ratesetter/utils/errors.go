package utils

import "github.com/cockroachdb/errors"

var (
	// ErrInvalidIssuer is returned when an invalid block is passed to the rate setter.
	ErrInvalidIssuer = errors.New("block not issued by local node")

	// ErrBlockWasNotScheduledInTime is returned if a block did not get issued within the defined await time.
	ErrBlockWasNotScheduledInTime = errors.New("block could not be scheduled in time")

	// ErrStopped is returned when a block is passed to a stopped rate setter.
	ErrStopped = errors.New("rate setter stopped")
)
