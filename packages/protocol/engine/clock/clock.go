package clock

import (
	"fmt"
	"time"
)

// Clock is a clock that is used to derive some Time parameters from the Tangle.
type Clock struct {
	Events *Events

	lastAccepted  timeUpdate
	lastConfirmed timeUpdate
}

// New creates a new Clock with the given genesisTime.
func New() (clock *Clock) {
	return &Clock{
		Events: NewEvents(),
	}
}

// AcceptedTime returns the Time of the last accepted Block.
func (c *Clock) AcceptedTime() (acceptedTime time.Time) {
	c.lastAccepted.RLock()
	defer c.lastAccepted.RUnlock()

	return c.lastAccepted.Time
}

// SetAcceptedTime sets the Time of the last accepted Block.
func (c *Clock) SetAcceptedTime(acceptedTime time.Time) (updated bool) {
	now := time.Now()
	if updated = c.updateTime(&c.lastAccepted, now, acceptedTime); updated {
		c.Events.AcceptanceTimeUpdated.Trigger(&TimeUpdateEvent{
			NewTime:    acceptedTime,
			UpdateTime: now,
		})
	}

	return
}

// RelativeAcceptedTime returns the real-Time adjusted version of the Time of the last accepted Block.
func (c *Clock) RelativeAcceptedTime() (relativeAcceptedTime time.Time) {
	c.lastAccepted.RLock()
	defer c.lastAccepted.RUnlock()

	return c.lastAccepted.Time.Add(time.Since(c.lastAccepted.Updated))
}

// ConfirmedTime returns the Time of the last confirmed Block.
func (c *Clock) ConfirmedTime() (confirmedTime time.Time) {
	c.lastConfirmed.RLock()
	defer c.lastConfirmed.RUnlock()

	return c.lastConfirmed.Time
}

// SetConfirmedTime sets the Time of the last confirmed Block.
func (c *Clock) SetConfirmedTime(confirmedTime time.Time) (updated bool) {
	now := time.Now()
	if updated = c.updateTime(&c.lastConfirmed, now, confirmedTime); updated {
		c.Events.ConfirmedTimeUpdated.Trigger(&TimeUpdateEvent{
			NewTime:    confirmedTime,
			UpdateTime: now,
		})
	}

	return
}

// RelativeConfirmedTime returns the real-Time adjusted version of the Time of the last confirmed Block.
func (c *Clock) RelativeConfirmedTime() (relativeConfirmedTime time.Time) {
	c.lastConfirmed.RLock()
	defer c.lastConfirmed.RUnlock()

	return c.lastConfirmed.Time.Add(time.Since(c.lastConfirmed.Updated))
}

// updateTime updates the given Time parameter if the given Time larger than the current Time.
func (c *Clock) updateTime(t *timeUpdate, now, newTime time.Time) (updated bool) {
	t.Lock()
	defer t.Unlock()

	// the local wall clock should never be before the accepted Time unless we are eclipsed by malicious actors or our
	// own Time is clearly in the past
	if now.Before(newTime) {
		panic(fmt.Sprintf("tried to set Time is in the future. now: %s, newTime: %s", now.String(), newTime.String()))
	}

	if updated = newTime.Unix() > t.Time.Unix(); updated {
		t.Time = newTime
		t.Updated = now
	}

	return
}
