package clock

import (
	"fmt"
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
	now := time.Now()
	if updated = c.updateTime(now, acceptedTime, &c.lastAcceptedTime, &c.lastAcceptedTimeUpdated); updated {
		c.Events.AcceptanceTimeUpdated.Trigger(&TimeUpdateEvent{
			NewTime:    acceptedTime,
			UpdateTime: now,
		})
	}

	return
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
	now := time.Now()
	if updated = c.updateTime(now, confirmedTime, &c.lastConfirmedTime, &c.lastConfirmedTimeUpdated); updated {
		c.Events.ConfirmedTimeUpdated.Trigger(&TimeUpdateEvent{
			NewTime:    confirmedTime,
			UpdateTime: now,
		})
	}

	return
}

// RelativeConfirmedTime returns the real-time adjusted version of the time of the last confirmed Block.
func (c *Clock) RelativeConfirmedTime() (relativeConfirmedTime time.Time) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastConfirmedTime.Add(time.Since(c.lastConfirmedTimeUpdated))
}

// updateTime updates the given time parameter if the given time larger than the current time.
func (c *Clock) updateTime(now, newTime time.Time, param, updatedParam *time.Time) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// the local wall clock should never be before the accepted time unless we are eclipsed by malicious actors or our
	// own time is clearly in the past
	if now.Before(newTime) {
		panic(fmt.Sprintf("tried to set time is in the future. now: %s, newTime: %s", now.String(), newTime.String()))
	}

	//if updated = newTime.After(*param); updated {
	if updated = newTime.Unix() > (*param).Unix(); updated {
		*param = newTime
		*updatedParam = now
	}

	return
}
