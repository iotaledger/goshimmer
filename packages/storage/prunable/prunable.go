package prunable

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
)

const (
	blocksPrefix byte = iota
	rootBlocksPrefix
	attestationsPrefix
	ledgerStateDiffsPrefix
)

type Prunable struct {
	Blocks           *Blocks
	RootBlocks       *RootBlocks
	Attestations     func(index slot.Index) kvstore.KVStore
	LedgerStateDiffs func(index slot.Index) kvstore.KVStore
}

func New(dbManager *database.Manager) (newPrunable *Prunable) {
	return &Prunable{
		Blocks:           NewBlocks(dbManager, blocksPrefix),
		RootBlocks:       NewRootBlocks(dbManager, rootBlocksPrefix),
		Attestations:     lo.Bind([]byte{attestationsPrefix}, dbManager.Get),
		LedgerStateDiffs: lo.Bind([]byte{ledgerStateDiffsPrefix}, dbManager.Get),
	}
}
