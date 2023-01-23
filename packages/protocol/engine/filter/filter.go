package filter

import (
	"errors"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// Filter filters blocks.
type Filter struct {
	Events *Events

	optsMinCommittableEpochAge time.Duration
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

	if block.Commitment().Index() > block.ID().Index()-epoch.Index(int64(f.optsMinCommittableEpochAge.Seconds())/epoch.Duration) {
		f.Events.BlockFiltered.Trigger(&BlockFilteredEvent{
			Block:  block,
			Reason: errors.New("a block cannot commit to an epoch that cannot objectively be committable yet"),
		})
		return
	}

	f.Events.BlockAllowed.Trigger(block)
}

// WithMinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func WithMinCommittableEpochAge(d time.Duration) options.Option[Filter] {
	return func(filter *Filter) {
		filter.optsMinCommittableEpochAge = d
	}
}
