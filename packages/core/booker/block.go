package booker

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

type Block struct {
	booked bool

	*tangle.Block
}

func NewBlock(block *tangle.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block: block,
	}, opts)
}

func (b *Block) IsBooked() (isBooked bool) {
	b.RLock()
	defer b.RUnlock()

	return b.booked
}

func (b *Block) setBooked() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.booked; wasUpdated {
		b.booked = true
	}

	return
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithBooked is a constructor Option for Blocks that initializes the given block with a specific missing flag.
func WithBooked(missing bool) options.Option[Block] {
	return func(block *Block) {
		block.booked = missing
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
