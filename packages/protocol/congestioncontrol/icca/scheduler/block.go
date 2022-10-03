package scheduler

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Block struct {
	scheduled bool
	skipped   bool
	dropped   bool

	*virtualvoting.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(virtualVotingBlock *virtualvoting.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: virtualVotingBlock,
	}, opts)
}

func NewRootBlock(id models.BlockID) (rootBlock *Block) {
	return NewBlock(
		virtualvoting.NewRootBlock(id),
		WithScheduled(true),
		WithSkipped(false),
		WithDiscarded(false),
	)
}

func (b *Block) IsScheduled() bool {
	b.RLock()
	defer b.RUnlock()

	return b.scheduled
}

func (b *Block) SetScheduled() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.scheduled; wasUpdated {
		b.scheduled = true
	}

	return
}

func (b *Block) IsDropped() bool {
	b.RLock()
	defer b.RUnlock()

	return b.dropped
}

func (b *Block) SetDropped() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.dropped; wasUpdated {
		b.dropped = true
	}

	return
}

func (b *Block) IsSkipped() bool {
	b.RLock()
	defer b.RUnlock()

	return b.skipped
}

func (b *Block) SetSkipped() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.skipped; wasUpdated {
		b.skipped = true
	}

	return
}

func NewRootBlock(blockID models.BlockID) *Block {
	virtualVotingBlock := virtualvoting.NewRootBlock(blockID)

	return NewBlock(virtualVotingBlock, WithScheduled(true))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithScheduled(scheduled bool) options.Option[Block] {
	return func(b *Block) {
		b.scheduled = scheduled
	}
}

func WithDiscarded(discarded bool) options.Option[Block] {
	return func(b *Block) {
		b.dropped = discarded
	}
}

func WithSkipped(skipped bool) options.Option[Block] {
	return func(b *Block) {
		b.skipped = skipped
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Blocks ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Blocks represents a collection of Block.
type Blocks = *set.AdvancedSet[*Block]

// NewBlocks returns a new Block collection with the given elements.
func NewBlocks(blocks ...*Block) (newBlocks Blocks) {
	return set.NewAdvancedSet(blocks...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
