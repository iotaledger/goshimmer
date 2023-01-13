package blockissuer

import (
	"github.com/pkg/errors"
)

var (
	// ErrBlockWasNotBookedInTime is returned if a block did not get booked within the defined await time.
	ErrBlockWasNotBookedInTime = errors.New("block could not be booked in time")

	// ErrBlockWasNotScheduledInTime is returned if a block did not get issued within the defined await time.
	ErrBlockWasNotScheduledInTime = errors.New("block could not be scheduled in time")

	// ErrNotBootstraped is returned if a block cannot be issued because the node is not bootstrapped.
	ErrNotBootstraped = errors.New("not bootstrapped")
)
