package filter

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// Filter filters blocks.
type Filter struct {
	Events *Events
}

// New creates a new Filter.
func New(opts ...options.Option[Filter]) (inbox *Filter) {
	return options.Apply(&Filter{
		Events: NewEvents(),
	}, opts)
}

// ProcessReceivedBlock processes block from the given source.
func (f *Filter) ProcessReceivedBlock(block *models.Block, source identity.ID) {
	// fill heuristic + check if block is valid
	// ...

	f.Events.BlockAllowed.Trigger(block)
}
