package scheduler

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Block struct {
	scheduled bool
	skipped   bool
	discarded bool

	*virtualvoting.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(virtualVotingBlock *virtualvoting.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: virtualVotingBlock,
	}, opts)
}

func (b *Block) Scheduled() bool {
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

func (b *Block) Dropped() bool {
	b.RLock()
	defer b.RUnlock()

	return b.discarded
}

func (b *Block) SetDiscarded() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.discarded; wasUpdated {
		b.discarded = true
	}

	return
}

func (b *Block) Skipped() bool {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithScheduled(scheduled bool) options.Option[Block] {
	return func(b *Block) {
		b.scheduled = scheduled
	}
}

func WithDiscarded(discarded bool) options.Option[Block] {
	return func(b *Block) {
		b.discarded = discarded
	}
}

func WithSkipped(skipped bool) options.Option[Block] {
	return func(b *Block) {
		b.skipped = skipped
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
