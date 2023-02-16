package prunable

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
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
	Attestations     func(index epoch.Index) kvstore.KVStore
	LedgerStateDiffs func(index epoch.Index) kvstore.KVStore
}

func New(dbManager *database.Manager) (newPrunable *Prunable) {
	return &Prunable{
		Blocks:           NewBlocks(dbManager, blocksPrefix),
		RootBlocks:       NewRootBlocks(dbManager, rootBlocksPrefix),
		Attestations:     lo.Bind([]byte{attestationsPrefix}, dbManager.Get),
		LedgerStateDiffs: lo.Bind([]byte{ledgerStateDiffsPrefix}, dbManager.Get),
	}
}
