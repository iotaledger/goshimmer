package instancemanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
)

type Events struct {
	InvalidBlockReceived *event.Linkable[*p2p.Neighbor, Events, *Events]

	Instance *instance.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		InvalidBlockReceived: event.NewLinkable[*p2p.Neighbor, Events](),

		Instance: instance.NewEvents(),
	}
})
