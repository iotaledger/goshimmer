package blockgadget

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with OTV related metadata.
type Block struct {
	accepted           bool
	weaklyAccepted     bool
	confirmed          bool
	weaklyConfirmed    bool
	acceptanceQueued   bool
	confirmationQueued bool

	*booker.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(virtualVotingBlock *booker.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: virtualVotingBlock,
	}, opts)
}

// IsAccepted returns true if the Block was accepted.
func (b *Block) IsAccepted() bool {
	b.RLock()
	defer b.RUnlock()

	return b.accepted || b.weaklyAccepted
}

// IsStronglyAccepted returns true if the Block was accepted through strong children.
func (b *Block) IsStronglyAccepted() bool {
	b.RLock()
	defer b.RUnlock()

	return b.accepted
}

// SetAccepted sets the Block as accepted.
func (b *Block) SetAccepted(weakly bool) (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if weakly {
		if wasUpdated = !b.weaklyAccepted; wasUpdated {
			b.weaklyAccepted = true
		}

		// return true only if block was neither strongly and weakly accepted before
		return wasUpdated && !b.accepted
	}

	if wasUpdated = !b.accepted; wasUpdated {
		b.accepted = true
	}

	// return true only if block was neither strongly and weakly accepted before
	return wasUpdated && !b.weaklyAccepted
}

func (b *Block) IsConfirmed() bool {
	b.RLock()
	defer b.RUnlock()

	return b.confirmed || b.weaklyConfirmed
}

func (b *Block) IsStronglyConfirmed() bool {
	b.RLock()
	defer b.RUnlock()

	return b.confirmed
}

func (b *Block) SetConfirmed(weakly bool) (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if weakly {
		if wasUpdated = !b.weaklyConfirmed; wasUpdated {
			b.weaklyConfirmed = true
		}

		// return true only if block was neither strongly and weakly confirmed before
		return wasUpdated && !b.confirmed
	}

	if wasUpdated = !b.confirmed; wasUpdated {
		b.confirmed = true
	}

	// return true only if block was neither strongly and weakly confirmed before
	return wasUpdated && !b.weaklyConfirmed
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

func NewRootBlock(blockID models.BlockID, slotTimeProvider *slot.TimeProvider) *Block {
	virtualVotingBlock := booker.NewRootBlock(blockID, slotTimeProvider)

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
