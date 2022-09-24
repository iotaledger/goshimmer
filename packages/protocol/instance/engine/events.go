package engine

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

type Events struct {
	Tangle    *tangle.Events
	Consensus *consensus.Events
	Ledger    *ledger.Events

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		Tangle:    tangle.NewEvents(),
		Consensus: consensus.NewEvents(),
		Ledger:    ledger.NewEvents(),
	}
})
