package booker

import (
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

type Block struct {
	ready  bool
	booked bool

	*tangle.Block
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
