package filter

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// Filter filters blocks.
type Filter struct {
	Events *Events

	blockRetrieverFunc func(models.BlockID) (block *blockdag.Block, exists bool)
}

// New creates a new Filter.
func New(blockRetrieverFunc func(models.BlockID) (block *blockdag.Block, exists bool), opts ...options.Option[Filter]) (inbox *Filter) {
	return options.Apply(&Filter{
		Events: NewEvents(),

		blockRetrieverFunc: blockRetrieverFunc,
	}, opts)
}

// ProcessReceivedBlock processes block from the given source.
func (f *Filter) ProcessReceivedBlock(block *models.Block, source identity.ID) {
	if !f.isCommitmentMonotonic(block) {
		f.Events.BlockFiltered.Trigger(&BlockFilteredEvent{
			Block:  block,
			Reason: "commitment is not monotonic",
		})
		return
	}

	// fill heuristic + check if block is valid
	// ...

	f.Events.BlockAllowed.Trigger(block)
}

func (f *Filter) isCommitmentMonotonic(block *models.Block) (isMonotonic bool) {
	for parent := range block.ParentsByType(models.StrongParentType) {
		if parentBlock, exists := f.blockRetrieverFunc(parent); !exists || parentBlock.Commitment().Index() > block.Commitment().Index() {
			return false
		}
	}

	return true
}
