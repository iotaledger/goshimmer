package clock

import (
	"time"
)

// RelativeClock is a time value that monotonically advances with the system clock but that is anchored to a specific
// point in time.
type RelativeClock interface {
	// Time returns the time that this clock is anchored to.
	Time() time.Time

	// RelativeTime returns.
	RelativeTime() time.Time
}
