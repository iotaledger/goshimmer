package clock

import (
	"time"
)

// RelativeTime is a time value that monotonically advances with the system clock.
type RelativeTime interface {
	// Time returns the original time value.
	Time() time.Time

	// RelativeTime returns the time value after it has advanced with the system clock.
	RelativeTime() time.Time
}
