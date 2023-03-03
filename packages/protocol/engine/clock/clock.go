package clock

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/core/module"
)

// Clock is a clock that is used to derive some Time parameters from the Tangle.
type Clock interface {
	// Events returns a dictionary that contains all events that are triggered by the Clock.
	Events() *Events

	// AcceptedTime returns the time of the last accepted Block.
	AcceptedTime() time.Time

	// RelativeAcceptedTime returns the real-time adjusted version of the time of the last accepted Block.
	RelativeAcceptedTime() time.Time

	// ConfirmedTime returns the time of the last confirmed Block.
	ConfirmedTime() time.Time

	// RelativeConfirmedTime returns the real-time adjusted version of the time of the last confirmed Block.
	RelativeConfirmedTime() time.Time

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
