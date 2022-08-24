package acceptancegadget

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with OTV related metadata.
type Block struct {
	accepted bool

	*booker.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(bookerBlock *booker.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: bookerBlock,
	}, opts)
}

func (b *Block) Accepted() bool {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
