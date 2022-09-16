package virtualvoting

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/booker"
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

// region Blocks ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Blocks represents a collection of Block.
type Blocks = *set.AdvancedSet[*Block]

// NewBlocks returns a new Block collection with the given elements.
func NewBlocks(blocks ...*Block) (newBlocks Blocks) {
	return set.NewAdvancedSet(blocks...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////