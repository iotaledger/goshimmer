package prunable

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

const (
	blocksPrefix byte = iota
	rootBlocksPrefix
	activityLogPrefix
	ledgerStateDiffsPrefix
)

type Prunable struct {
	Blocks           *Blocks
	RootBlocks       *RootBlocks
	Attestors        func(index epoch.Index) kvstore.KVStore
	LedgerStateDiffs func(index epoch.Index) kvstore.KVStore
}

func New(database *database.Manager) (newPrunable *Prunable) {
	return &Prunable{
		Blocks:           NewBlocks(database, blocksPrefix),
		RootBlocks:       NewRootBlocks(database, rootBlocksPrefix),
		Attestors:        lo.Bind([]byte{activityLogPrefix}, database.Get),
		LedgerStateDiffs: lo.Bind([]byte{ledgerStateDiffsPrefix}, database.Get),
	}
}
