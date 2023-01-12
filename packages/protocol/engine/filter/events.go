package filter

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Events struct {
	BlockAllowed  *event.Linkable[*models.Block]
	BlockFiltered *event.Linkable[*BlockFilteredEvent]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		BlockAllowed:  event.NewLinkable[*models.Block](),
		BlockFiltered: event.NewLinkable[*BlockFilteredEvent](),
	}
})

// BlockFilteredEvent is the event that is triggered when a block is filtered, providing a reason why it was filtered.
type BlockFilteredEvent struct {
	Block  *models.Block
	Reason string
}
