package engine

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Events struct {
	Error *event.Linkable[error]

	Inbox               *inbox.Events
	Ledger              *ledger.Events
	Tangle              *tangle.Events
	Consensus           *consensus.Events
	Clock               *clock.Events
	EvictionManager     *eviction.Events
	NotarizationManager *notarization.Events
	BlockRequester      *eventticker.Events[models.BlockID]
	ManaTracker         *mana.Events

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		Error: event.NewLinkable[error](),

		Inbox:               inbox.NewEvents(),
		Ledger:              ledger.NewEvents(),
		Tangle:              tangle.NewEvents(),
		Consensus:           consensus.NewEvents(),
		Clock:               clock.NewEvents(),
		EvictionManager:     eviction.NewEvents(),
		NotarizationManager: notarization.NewEvents(),
		BlockRequester:      eventticker.NewEvents[models.BlockID](),
		ManaTracker:         mana.NewEvents(),
	}
})
