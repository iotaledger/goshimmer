package blockcreation

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
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
	TipAdded *event.Event[*virtualvoting.Block]

	// Fired when a tip is removed.
	TipRemoved *event.Event[*virtualvoting.Block]
}

func newTipManagerEvents() (new *TipManagerEvents) {
	return &TipManagerEvents{
		TipAdded:   event.New[*virtualvoting.Block](),
		TipRemoved: event.New[*virtualvoting.Block](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
