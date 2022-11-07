package filter

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Filter struct {
	Events *Events
}

func New(opts ...options.Option[Filter]) (inbox *Filter) {
	return options.Apply(&Filter{
		Events: NewEvents(),
	}, opts)
}

func (i *Filter) ProcessReceivedBlock(block *models.Block, source identity.ID) {
	// fill heuristic + check if block is valid
	// ...

	i.Events.BlockReceived.Trigger(block)
}
