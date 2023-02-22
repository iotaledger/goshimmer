package blockfactory

import (
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/runtime/event"
)

// Events represents events happening on a block factory.
type Events struct {
	// Fired when a block is built including tips, sequence number and other metadata.
	BlockConstructed *event.Event1[*models.Block]

	// Fired when an error occurred.
	Error *event.Event1[error]

	event.Group[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var newEvents = event.CreateGroupConstructor(func() (newEvents *Events) {
	return &Events{
		BlockConstructed: event.New1[*models.Block](),
		Error:            event.New1[error](),
	}
})
