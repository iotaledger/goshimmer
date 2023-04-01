package scheduler

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block that is scheduled by the scheduler.
type Block struct {
	scheduled bool
	skipped   bool
	dropped   bool

	*booker.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(virtualVotingBlock *booker.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: virtualVotingBlock,
	}, opts)
}

// NewRootBlock creates a new root Block.
func NewRootBlock(id models.BlockID, slotTimeProvider *slot.TimeProvider) (rootBlock *Block) {
	return NewBlock(
		booker.NewRootBlock(id, slotTimeProvider),
		WithScheduled(true),
		WithSkipped(false),
		WithDiscarded(false),
	)
}

// IsScheduled returns true if the Block is scheduled.
func (b *Block) IsScheduled() bool {
	b.RLock()
	defer b.RUnlock()

	return b.scheduled
}

// SetScheduled sets the scheduled flag of the Block.
func (b *Block) SetScheduled() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.scheduled; wasUpdated {
		b.scheduled = true
	}

	return
}

// IsDropped returns true if the Block is dropped.
func (b *Block) IsDropped() bool {
	b.RLock()
	defer b.RUnlock()

	return b.dropped
}

// SetDropped sets the dropped flag of the Block.
func (b *Block) SetDropped() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.dropped; wasUpdated {
		b.dropped = true
	}

	return
}

// IsSkipped returns true if the Block is skipped.
func (b *Block) IsSkipped() bool {
	b.RLock()
	defer b.RUnlock()

	return b.skipped
}

// SetSkipped sets the skipped flag of the Block.
func (b *Block) SetSkipped() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.skipped; wasUpdated {
		b.skipped = true
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithScheduled sets the scheduled flag of the Block.
func WithScheduled(scheduled bool) options.Option[Block] {
	return func(b *Block) {
		b.scheduled = scheduled
	}
}

// WithDiscarded sets the discarded flag of the Block.
func WithDiscarded(discarded bool) options.Option[Block] {
	return func(b *Block) {
		b.dropped = discarded
	}
}

// WithSkipped sets the skipped flag of the Block.
func WithSkipped(skipped bool) options.Option[Block] {
	return func(b *Block) {
		b.skipped = skipped
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Blocks ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Blocks represents a collection of Block.
type Blocks = *advancedset.AdvancedSet[*Block]

// NewBlocks returns a new Block collection with the given elements.
func NewBlocks(blocks ...*Block) (newBlocks Blocks) {
	return advancedset.New(blocks...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
