package clock

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
)

// Clock is a component that provides different notions of time for the Engine.
type Clock interface {
	// AcceptanceTime returns a notion of time that is anchored to the latest accepted block.
	AcceptanceTime() *AnchoredTime

	// ConfirmationTime returns a notion of time that is anchored to the latest confirmed block.
	ConfirmationTime() *AnchoredTime

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
