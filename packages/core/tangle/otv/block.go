package otv

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with OTV related metadata.
type Block struct {
	subjectivelyInvalid bool

	*booker.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(bookerBlock *booker.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: bookerBlock,
	}, opts)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
