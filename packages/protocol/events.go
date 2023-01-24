package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
)

type Events struct {
	InvalidBlockReceived   *event.Linkable[identity.ID]
	CandidateEngineCreated *event.Linkable[*enginemanager.EngineInstance]
	MainEngineSwitched     *event.Linkable[*enginemanager.EngineInstance]
	Error                  *event.Linkable[error]

	Network           *network.Events
	Engine            *engine.Events
	CongestionControl *congestioncontrol.Events
	TipManager        *tipmanager.Events
	ChainManager      *chainmanager.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		InvalidBlockReceived:   event.NewLinkable[identity.ID](),
		CandidateEngineCreated: event.NewLinkable[*enginemanager.EngineInstance](),
		MainEngineSwitched:     event.NewLinkable[*enginemanager.EngineInstance](),
		Error:                  event.NewLinkable[error](),

		Network:           network.NewEvents(),
		Engine:            engine.NewEvents(),
		CongestionControl: congestioncontrol.NewEvents(),
		TipManager:        tipmanager.NewEvents(),
		ChainManager:      chainmanager.NewEvents(),
	}
})
