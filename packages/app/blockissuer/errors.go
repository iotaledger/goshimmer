package blockissuer

import (
	"github.com/cockroachdb/errors"
)

var (
	// ErrBlockWasNotBookedInTime is returned if a block did not get booked within the defined await time.
	ErrBlockWasNotBookedInTime = errors.New("block could not be booked in time")

	// ErrBlockWasNotScheduledInTime is returned if a block did not get issued within the defined await time.
	ErrBlockWasNotScheduledInTime = errors.New("block could not be scheduled in time")
)
