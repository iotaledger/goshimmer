package prunable

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
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
	ActiveNodes      *ActiveNodes
	LedgerStateDiffs *LedgerStateDiffs
}

func New(database *database.Manager) (newPrunable *Prunable) {
	return &Prunable{
		Blocks:           NewBlocks(database, blocksPrefix),
		RootBlocks:       NewRootBlocks(database, rootBlocksPrefix),
		ActiveNodes:      NewActiveNodes(database, activityLogPrefix),
		LedgerStateDiffs: NewLedgerStateDiffs(database, ledgerStateDiffsPrefix),
	}
}
