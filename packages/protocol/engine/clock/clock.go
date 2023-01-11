package clock

import (
	"time"
)

// Clock is a clock that is used to derive some Time parameters from the Tangle.
type Clock struct {
	Events *Events

	lastAccepted  *timeUpdate
	lastConfirmed *timeUpdate
}

// New creates a new Clock with the given genesisTime.
func New() (clock *Clock) {
	return &Clock{
		lastAccepted:  &timeUpdate{},
		lastConfirmed: &timeUpdate{},

		Events: NewEvents(),
	}
}

// AcceptedTime returns the Time of the last accepted Block.
func (c *Clock) AcceptedTime() (acceptedTime time.Time) {
	return c.lastAccepted.Time()
}

// SetAcceptedTime sets the Time of the last accepted Block.
func (c *Clock) SetAcceptedTime(acceptedTime time.Time) (updated bool) {
	now := time.Now()
	if updated = c.lastAccepted.Update(now, acceptedTime); updated {
		c.Events.AcceptanceTimeUpdated.Trigger(&TimeUpdateEvent{
			NewTime:    acceptedTime,
			UpdateTime: now,
		}, "acceptance time updated")
	}

	return
}

// RelativeAcceptedTime returns the real-Time adjusted version of the Time of the last accepted Block.
func (c *Clock) RelativeAcceptedTime() (relativeAcceptedTime time.Time) {
	return c.lastAccepted.RelativeTime()
}

// ConfirmedTime returns the Time of the last confirmed Block.
func (c *Clock) ConfirmedTime() (confirmedTime time.Time) {
	return c.lastConfirmed.Time()
}

// SetConfirmedTime sets the Time of the last confirmed Block.
func (c *Clock) SetConfirmedTime(confirmedTime time.Time) (updated bool) {
	now := time.Now()
	if updated = c.lastConfirmed.Update(now, confirmedTime); updated {
		c.Events.ConfirmedTimeUpdated.Trigger(&TimeUpdateEvent{
			NewTime:    confirmedTime,
			UpdateTime: now,
		}, "confirmed time updated")
	}

	return
}

// RelativeConfirmedTime returns the real-Time adjusted version of the Time of the last confirmed Block.
func (c *Clock) RelativeConfirmedTime() (relativeConfirmedTime time.Time) {
	return c.lastConfirmed.RelativeTime()
}
