package blocktime

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/runtime/event"
)

// RelativeTime is a time value that monotonically advances with the system clock.
type RelativeTime struct {
	// OnUpdated is triggered when the time is updated.
	OnUpdated *event.Event1[time.Time]

	// time is the original time value.
	time time.Time

	// timeUpdateOffset is the offset of the local clock when the time was updated.
	timeUpdateOffset time.Time

	// mutex is used to synchronize access to the time value.
	mutex sync.RWMutex
}

// NewRelativeTime creates a new RelativeTime.
func NewRelativeTime() *RelativeTime {
	return &RelativeTime{
		OnUpdated: event.New1[time.Time](),
	}
}

// Time returns the original time value.
func (c *RelativeTime) Time() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.time
}

// RelativeTime returns the time value after it has advanced with the system clock.
func (c *RelativeTime) RelativeTime() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.time.Add(time.Since(c.timeUpdateOffset))
}

// Set sets the time value if the given time is larger than the current time (resetting monotonicity of the relative
// time).
func (c *RelativeTime) Set(newTime time.Time) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if newTime.Before(c.time) {
		return false
	}

	c.timeUpdateOffset = time.Now()
	c.time = newTime

	c.OnUpdated.Trigger(c.time)

	return true
}

// Advance advances the time value if the given time is larger than the current time (maintaining monotonicity of the
// relative time).
func (c *RelativeTime) Advance(newTime time.Time) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if newTime.Before(c.time) {
		return false
	}

	c.timeUpdateOffset = c.determineTimeUpdateOffset(newTime)
	c.time = newTime

	c.OnUpdated.Trigger(c.time)

	return true
}

// determineTimeUpdateOffset determines the new timeUpdateOffset that is in sync with the monotonic clock.
func (c *RelativeTime) determineTimeUpdateOffset(newTime time.Time) time.Time {
	diff := time.Since(c.timeUpdateOffset)

	// if the new time lags behind the monotonic time, we adjust the offset to prevent the clock from going backwards.
	if lag := newTime.Sub(c.time.Add(diff)); lag < 0 {
		diff += lag
	}

	return c.timeUpdateOffset.Add(diff)
}
