package engine

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tipmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

type Events struct {
	Ledger              *ledger.Events
	Tangle              *tangle.Events
	Consensus           *consensus.Events
	Clock               *clock.Events
	CongestionControl   *congestioncontrol.Events
	TipManager          *tipmanager.Events
	EvictionManager     *eviction.Events
	NotarizationManager *notarization.Events

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		Ledger:              ledger.NewEvents(),
		Tangle:              tangle.NewEvents(),
		Consensus:           consensus.NewEvents(),
		Clock:               clock.NewEvents(),
		CongestionControl:   congestioncontrol.NewEvents(),
		TipManager:          tipmanager.NewEvents(),
		EvictionManager:     eviction.NewEvents(),
		NotarizationManager: notarization.NewEvents(),
	}
})
