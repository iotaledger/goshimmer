package blockgadget

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with OTV related metadata.
type Block struct {
	accepted           bool
	confirmed          bool
	acceptanceQueued   bool
	confirmationQueued bool

	*virtualvoting.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(virtualVotingBlock *virtualvoting.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: virtualVotingBlock,
	}, opts)
}

// IsAccepted returns true if the Block was accepted.
func (b *Block) IsAccepted() bool {
	b.RLock()
	defer b.RUnlock()

	return b.accepted
}

// SetAccepted sets the Block as accepted.
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

func (b *Block) IsAcceptanceQueued() bool {
	b.RLock()
	defer b.RUnlock()

	return b.acceptanceQueued
}

func (b *Block) SetAcceptanceQueued() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.acceptanceQueued; wasUpdated {
		b.acceptanceQueued = true
	}

	return
}

func (b *Block) IsConfirmationQueued() bool {
	b.RLock()
	defer b.RUnlock()

	return b.confirmationQueued
}

func (b *Block) SetConfirmationQueued() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.confirmationQueued; wasUpdated {
		b.confirmationQueued = true
	}

	return
}

func NewRootBlock(blockID models.BlockID, epochTimeProvider *epoch.TimeProvider) *Block {
	virtualVotingBlock := virtualvoting.NewRootBlock(blockID, epochTimeProvider)

	return NewBlock(virtualVotingBlock, WithAccepted(true), WithConfirmed(true))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAccepted(accepted bool) options.Option[Block] {
	return func(b *Block) {
		b.accepted = accepted
	}
}

func WithConfirmed(confirmed bool) options.Option[Block] {
	return func(b *Block) {
		b.confirmed = confirmed
	}
}

func WithQueued(queued bool) options.Option[Block] {
	return func(b *Block) {
		b.acceptanceQueued = queued
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
