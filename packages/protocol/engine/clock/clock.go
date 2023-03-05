package clock

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
)

// Clock is a clock that is used to derive some Time parameters from the Tangle.
type Clock interface {
	AcceptanceTime() *AnchoredTime

	ConfirmationTime() *AnchoredTime

	// Events returns a dictionary that contains all events that are triggered by the Clock.
	Events() *Events

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
