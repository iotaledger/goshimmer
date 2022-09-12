package blockfactory

import (
	"github.com/iotaledger/hive.go/core/generics/event"

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
func newBlockFactoryEvents() (new *Events) {
	return &Events{
		BlockConstructed: event.New[*models.Block](),
		Error:            event.New[error](),
	}
}
