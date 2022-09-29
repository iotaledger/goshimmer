package acceptance

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with OTV related metadata.
type Block struct {
	accepted bool

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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
