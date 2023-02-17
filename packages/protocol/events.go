package protocol

import (
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/runtime/event"
)

type Events struct {
	InvalidBlockReceived     *event.Event1[identity.ID]
	CandidateEngineActivated *event.Event1[*enginemanager.EngineInstance]
	MainEngineSwitched       *event.Event1[*enginemanager.EngineInstance]
	Error                    *event.Event1[error]

	Network           *network.Events
	Engine            *engine.Events
	CongestionControl *congestioncontrol.Events
	TipManager        *tipmanager.Events
	ChainManager      *chainmanager.Events

	event.Group[Events, *Events]
}

var NewEvents = event.CreateGroupConstructor(func() (newEvents *Events) {
	return &Events{
		InvalidBlockReceived:     event.New1[identity.ID](),
		CandidateEngineActivated: event.New1[*enginemanager.EngineInstance](),
		MainEngineSwitched:       event.New1[*enginemanager.EngineInstance](),
		Error:                    event.New1[error](),

		Network:           network.NewEvents(),
		Engine:            engine.NewEvents(),
		CongestionControl: congestioncontrol.NewEvents(),
		TipManager:        tipmanager.NewEvents(),
		ChainManager:      chainmanager.NewEvents(),
	}
})
