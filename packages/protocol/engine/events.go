package engine

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/eventticker"
	"github.com/iotaledger/hive.go/runtime/event"
)

type Events struct {
	Error          *event.Event1[error]
	BlockProcessed *event.Event1[models.BlockID]

	EvictionState       *eviction.Events
	Filter              *filter.Events
	Ledger              *ledger.Events
	Tangle              *tangle.Events
	Consensus           *consensus.Events
	Clock               *clock.Events
	NotarizationManager *notarization.Events
	SlotMutations       *notarization.SlotMutationsEvents
	BlockRequester      *eventticker.Events[models.BlockID]

	event.Group[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.CreateGroupConstructor(func() (newEvents *Events) {
	return &Events{
		Error:               event.New1[error](),
		BlockProcessed:      event.New1[models.BlockID](),
		EvictionState:       eviction.NewEvents(),
		Filter:              filter.NewEvents(),
		Ledger:              ledger.NewEvents(),
		Tangle:              tangle.NewEvents(),
		Consensus:           consensus.NewEvents(),
		Clock:               clock.NewEvents(),
		NotarizationManager: notarization.NewEvents(),
		SlotMutations:       notarization.NewSlotMutationsEvents(),
		BlockRequester:      eventticker.NewEvents[models.BlockID](),
	}
})
