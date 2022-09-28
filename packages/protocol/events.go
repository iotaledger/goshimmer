package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine"
)

type Events struct {
	InvalidBlockReceived *event.Linkable[identity.ID, Events, *Events]

	Engine *engine.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		Engine: engine.NewEvents(),
	}
})
