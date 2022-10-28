package acceptance

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with OTV related metadata.
type Block struct {
	accepted  bool
	confirmed bool
	queued    bool

	*virtualvoting.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(virtualVotingBlock *virtualvoting.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: virtualVotingBlock,
	}, opts)
}

func (b *Block) IsAccepted() bool {
	b.RLock()
	defer b.RUnlock()

	return b.accepted
}

func (b *Block) SetAccepted() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.accepted; wasUpdated {
		b.accepted = true
	}

	return
}

func (b *Block) IsConfirmed() bool {
	b.RLock()
	defer b.RUnlock()

	return b.confirmed
}

func (b *Block) SetConfirmed() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.confirmed; wasUpdated {
		b.confirmed = true
	}

	return
}

func (b *Block) IsQueued() bool {
	b.RLock()
	defer b.RUnlock()

	return b.queued
}

func (b *Block) SetQueued() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.queued; wasUpdated {
		b.queued = true
	}

	return
}

func NewRootBlock(blockID models.BlockID) *Block {
	virtualVotingBlock := virtualvoting.NewRootBlock(blockID)

	return NewBlock(virtualVotingBlock, WithAccepted(true))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAccepted(accepted bool) options.Option[Block] {
	return func(b *Block) {
		b.accepted = accepted
	}
}

func WithQueued(queued bool) options.Option[Block] {
	return func(b *Block) {
		b.queued = queued
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
