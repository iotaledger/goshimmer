package engine

import (
	"sync"
	"time"
)

// Clock is a clock that is used to derive some time parameters from the Tangle.
type Clock struct {
	lastAcceptedTime         time.Time
	lastAcceptedTimeUpdated  time.Time
	lastConfirmedTime        time.Time
	lastConfirmedTimeUpdated time.Time

	sync.RWMutex
}

// NewClock creates a new Clock with the given genesisTime.
func NewClock(genesisTime time.Time) (clock *Clock) {
	clock = new(Clock)
	clock.SetAcceptedTime(genesisTime)
	clock.SetConfirmedTime(genesisTime)

	return clock
}

// AcceptedTime returns the time of the last accepted Block.
func (c *Clock) AcceptedTime() (acceptedTime time.Time) {
	c.RLock()
	defer c.RUnlock()

	return c.lastAcceptedTime
}

// SetAcceptedTime sets the time of the last accepted Block.
func (c *Clock) SetAcceptedTime(acceptedTime time.Time) (updated bool) {
	return c.updateTime(acceptedTime, &c.lastAcceptedTime, &c.lastAcceptedTimeUpdated)
}

// RelativeAcceptedTime returns the real-time adjusted version of the time of the last accepted Block.
func (c *Clock) RelativeAcceptedTime() (relativeAcceptedTime time.Time) {
	c.RLock()
	defer c.RUnlock()

	return c.lastAcceptedTime.Add(time.Since(c.lastAcceptedTimeUpdated))
}

// ConfirmedTime returns the time of the last confirmed Block.
func (c *Clock) ConfirmedTime() (confirmedTime time.Time) {
	c.RLock()
	defer c.RUnlock()

	return c.lastConfirmedTime
}

// SetConfirmedTime sets the time of the last confirmed Block.
func (c *Clock) SetConfirmedTime(confirmedTime time.Time) (updated bool) {
	return c.updateTime(confirmedTime, &c.lastConfirmedTime, &c.lastConfirmedTimeUpdated)
}

// RelativeConfirmedTime returns the real-time adjusted version of the time of the last confirmed Block.
func (c *Clock) RelativeConfirmedTime() (relativeConfirmedTime time.Time) {
	c.RLock()
	defer c.RUnlock()

	return c.lastConfirmedTime.Add(time.Since(c.lastConfirmedTimeUpdated))
}

// updateTime updates the given time parameter if the given time larger than the current time.
func (c *Clock) updateTime(newTime time.Time, param, updatedParam *time.Time) (updated bool) {
	c.Lock()
	defer c.Unlock()

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