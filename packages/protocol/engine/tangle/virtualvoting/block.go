package virtualvoting

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with VirtualVoting related metadata.
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

func NewRootBlock(id models.BlockID, opts ...options.Option[models.Block]) (rootBlock *Block) {
	return NewBlock(
		booker.NewRootBlock(id, opts...),
	)
}

func (b *Block) IsSubjectivelyInvalid() bool {
	b.RLock()
	defer b.RUnlock()

	return b.subjectivelyInvalid
}

func (b *Block) SetSubjectivelyInvalid(bool) (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.subjectivelyInvalid; wasUpdated {
		b.subjectivelyInvalid = true
	}

	return
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
