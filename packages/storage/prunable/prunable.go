package prunable

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
)

const (
	blocksPrefix byte = iota
	entryPointsPrefix
	activityLogPrefix
	ledgerStateDiffsPrefix
)

type Prunable struct {
	Blocks           *Blocks
	EntryPoints      *EntryPoints
	ActiveNodes      *ActiveNodes
	LedgerStateDiffs *LedgerStateDiffs
}

func New(database *database.Manager) (newPrunable *Prunable) {
	return &Prunable{
		Blocks:           NewBlocks(database, blocksPrefix),
		EntryPoints:      NewSolidEntryPoints(database, entryPointsPrefix),
		ActiveNodes:      NewActiveNodes(database, activityLogPrefix),
		LedgerStateDiffs: NewLedgerStateDiffs(database, ledgerStateDiffsPrefix),
	}
}
