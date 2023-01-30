package filter

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Events struct {
	BlockFiltered *event.Linkable[*BlockFilteredEvent]
	BlockAllowed  *event.Linkable[*models.Block]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		BlockFiltered: event.NewLinkable[*BlockFilteredEvent](),
		BlockAllowed:  event.NewLinkable[*models.Block](),
	}
})

type BlockFilteredEvent struct {
	Block  *models.Block
	Reason error
}
