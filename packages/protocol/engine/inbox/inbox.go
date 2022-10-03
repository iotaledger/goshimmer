package inbox

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Inbox struct {
	Events *Events
}

func New(opts ...options.Option[Inbox]) (inbox *Inbox) {
	return options.Apply(&Inbox{
		Events: NewEvents(),
	}, opts)
}

func (i Inbox) ProcessReceivedBlock(block *models.Block, source identity.ID) {
	// fill heuristic + check if block is valid
	// ...

	i.Events.BlockReceived.Trigger(block)
}
