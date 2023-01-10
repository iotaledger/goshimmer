package filter

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Events struct {
	BlockAllowed *event.Linkable[*models.Block]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		BlockAllowed: event.NewLinkable[*models.Block](),
	}
})
