package tip

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
)

// Events represents events happening on the Manager.
type Events struct {
	// Fired when a tip is added.
	TipAdded *event.Event[*scheduler.Block]

	// Fired when a tip is removed.
	TipRemoved *event.Event[*scheduler.Block]
}

func newEvents() (new *Events) {
	return &Events{
		TipAdded:   event.New[*scheduler.Block](),
		TipRemoved: event.New[*scheduler.Block](),
	}
}
