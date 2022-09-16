package inbox

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/models"
)

type Events struct {
	BlockReceived *event.Linkable[*models.Block, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		BlockReceived: event.NewLinkable[*models.Block, Events, *Events](),
	}
})
