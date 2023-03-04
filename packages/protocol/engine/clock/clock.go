package clock

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/core/module"
)

// Clock is a clock that is used to derive some Time parameters from the Tangle.
type Clock interface {
	// AcceptedTime returns the time of the last accepted Block.
	AcceptedTime() time.Time

	// RelativeAcceptedTime returns the real-time adjusted version of the time of the last accepted Block.
	RelativeAcceptedTime() time.Time

	// SetAcceptedTime sets the time of the last accepted Block.
	SetAcceptedTime(time.Time)

	// ConfirmedTime returns the time of the last confirmed Block.
	ConfirmedTime() time.Time

	// RelativeConfirmedTime returns the real-time adjusted version of the time of the last confirmed Block.
	RelativeConfirmedTime() time.Time

	// SetConfirmedTime sets the time of the last confirmed Block.
	SetConfirmedTime(time.Time)

	// Events returns a dictionary that contains all events that are triggered by the Clock.
	Events() *Events

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
