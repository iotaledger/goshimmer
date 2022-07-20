package tangle

import (
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

type Tangle struct {
	diskStorage any
	memStorage  *MemStorage
	ledger      *ledger.Ledger
	dbManager   database.Manager
}

func (t *Tangle) Attach(block *Block) {
	// abort if too old
	if false {
		return
	}

	blockMetadata := NewBlockMetadata(block)
	if !t.memStorage.PutBlockMetadata(blockMetadata) {
		return
	}

	// t.diskStorage.Put(block)

}
