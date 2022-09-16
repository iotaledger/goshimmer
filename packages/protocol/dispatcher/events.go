package dispatcher

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/network/p2p"
)

type Events struct {
	InvalidBlockReceived *event.Linkable[*p2p.Neighbor, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		InvalidBlockReceived: event.NewLinkable[*p2p.Neighbor, Events](),
	}
})
