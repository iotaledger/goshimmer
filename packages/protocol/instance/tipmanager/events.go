package tipmanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
)

// Events represents events happening on the TipManager.
type Events struct {
	// Fired when a tip is added.
	TipAdded *event.Linkable[*scheduler.Block, Events, *Events]

	// Fired when a tip is removed.
	TipRemoved *event.Linkable[*scheduler.Block, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		TipAdded:   event.NewLinkable[*scheduler.Block, Events](),
		TipRemoved: event.NewLinkable[*scheduler.Block, Events](),
	}
})
