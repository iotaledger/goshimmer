package inbox

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Events struct {
	BlockReceived *event.Linkable[*models.Block]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		BlockReceived: event.NewLinkable[*models.Block](),
	}
})
