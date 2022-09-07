package inbox

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/network"
)

type Inbox struct {
	Events *Events
}

func New(opts ...options.Option[Inbox]) (inbox *Inbox) {
	return options.Apply(&Inbox{
		Events: NewEvents(),
	}, opts)
}

func (i Inbox) ProcessBlockReceivedEvent(event *network.BlockReceivedEvent) {
	// fill heuristic + check if block is valid
	// ...

	i.Events.BlockReceived.Trigger(event.Block)
}
