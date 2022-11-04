package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
)

type Events struct {
	InvalidBlockReceived *event.Linkable[identity.ID]

	Engine            *engine.Events
	CongestionControl *congestioncontrol.Events
	TipManager        *tipmanager.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		Engine:            engine.NewEvents(),
		CongestionControl: congestioncontrol.NewEvents(),
		TipManager:        tipmanager.NewEvents(),
	}
})
