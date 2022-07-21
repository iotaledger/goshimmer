package tangle

import (
	"github.com/iotaledger/hive.go/generics/lo"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

type Tangle struct {
	memStorage *MemStorage
	ledger     *ledger.Ledger
	dbManager  database.Manager
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

	err := t.dbManager.Get(block.ID().EpochIndex, []byte{tangleold.PrefixBlock}).Set(block.IDBytes(), lo.PanicOnErr(block.Bytes()))
	if err != nil {
		panic(err)
	}

}
