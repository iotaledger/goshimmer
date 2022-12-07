package clock

import (
	"sync"
	"time"
)

// Clock is a clock that is used to derive some time parameters from the Tangle.
type Clock struct {
	Events *Events

	lastAcceptedTime         time.Time
	lastAcceptedTimeUpdated  time.Time
	lastConfirmedTime        time.Time
	lastConfirmedTimeUpdated time.Time
	mutex                    sync.RWMutex
}

// New creates a new Clock with the given genesisTime.
func New() (clock *Clock) {
	return &Clock{
		Events: NewEvents(),
	}
}

// AcceptedTime returns the time of the last accepted Block.
func (c *Clock) AcceptedTime() (acceptedTime time.Time) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastAcceptedTime
}

// SetAcceptedTime sets the time of the last accepted Block.
func (c *Clock) SetAcceptedTime(acceptedTime time.Time) (updated bool) {
	if updated = c.updateTime(acceptedTime, &c.lastAcceptedTime, &c.lastAcceptedTimeUpdated); updated {
		c.Events.AcceptanceTimeUpdated.Trigger(&TimeUpdate{
			NewTime:    c.lastAcceptedTime,
			UpdateTime: c.lastAcceptedTimeUpdated,
		})
	}
	return updated
}

// RelativeAcceptedTime returns the real-time adjusted version of the time of the last accepted Block.
func (c *Clock) RelativeAcceptedTime() (relativeAcceptedTime time.Time) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastAcceptedTime.Add(time.Since(c.lastAcceptedTimeUpdated))
}

// ConfirmedTime returns the time of the last confirmed Block.
func (c *Clock) ConfirmedTime() (confirmedTime time.Time) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastConfirmedTime
}

// SetConfirmedTime sets the time of the last confirmed Block.
func (c *Clock) SetConfirmedTime(confirmedTime time.Time) (updated bool) {
	if updated = c.updateTime(confirmedTime, &c.lastConfirmedTime, &c.lastConfirmedTimeUpdated); updated {
		c.Events.ConfirmedTimeUpdated.Trigger(&TimeUpdate{
			NewTime:    c.lastConfirmedTime,
			UpdateTime: c.lastConfirmedTimeUpdated,
		})
	}
	return updated
}

// RelativeConfirmedTime returns the real-time adjusted version of the time of the last confirmed Block.
func (c *Clock) RelativeConfirmedTime() (relativeConfirmedTime time.Time) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastConfirmedTime.Add(time.Since(c.lastConfirmedTimeUpdated))
}

// updateTime updates the given time parameter if the given time larger than the current time.
func (c *Clock) updateTime(newTime time.Time, param, updatedParam *time.Time) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// the local wall clock should never be before the accepted time unless we are eclipsed by malicious actors or our
	// own time is clearly in the past
	if time.Now().Before(newTime) {
		panic("accepted time is in the future")
	}

	if updated = newTime.After(*param); updated {
		*param = newTime
		*updatedParam = time.Now()
	}

	return
}
