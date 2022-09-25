package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
)

type Events struct {
	InvalidBlockReceived *event.Linkable[*p2p.Neighbor, Events, *Events]

	Instance *engine.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		Instance: engine.NewEvents(),
	}
})
