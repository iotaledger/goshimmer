package clock

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
)

// Clock is a component that provides different notions of time for the Engine that are aligned with the two notions of
// confirmation.
type Clock interface {
	// Accepted returns a notion of time that is anchored to the latest accepted block.
	Accepted() RelativeClock

	// Confirmed returns a notion of time that is anchored to the latest confirmed block.
	Confirmed() RelativeClock

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
