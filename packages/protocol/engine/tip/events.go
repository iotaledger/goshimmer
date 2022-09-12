package tip

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// Events represents events happening on a block factory.
type Events struct {
	// Fired when a block is built including tips, sequence number and other metadata.
	BlockConstructed *event.Event[*models.Block]

	// Fired when an error occurred.
	Error *event.Event[error]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		BlockConstructed: event.New[*models.Block](),
		Error:            event.New[error](),
	}
}

// region TipManagerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// TipManagerEvents represents events happening on the TipManager.
type TipManagerEvents struct {
	// Fired when a tip is added.
	TipAdded *event.Event[*scheduler.Block]

	// Fired when a tip is removed.
	TipRemoved *event.Event[*scheduler.Block]
}

func newTipManagerEvents() (new *TipManagerEvents) {
	return &TipManagerEvents{
		TipAdded:   event.New[*scheduler.Block](),
		TipRemoved: event.New[*scheduler.Block](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
