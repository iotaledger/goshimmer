package blockfactory

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// Events represents events happening on a block factory.
type Events struct {
	// Fired when a block is built including tips, sequence number and other metadata.
	BlockConstructed *event.Linkable[*models.Block]

	// Fired when an error occurred.
	Error *event.Linkable[error]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var newEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockConstructed: event.NewLinkable[*models.Block](),
		Error:            event.NewLinkable[error](),
	}
})
