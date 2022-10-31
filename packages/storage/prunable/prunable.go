package prunable

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
)

const (
	blocksPrefix byte = iota
	solidEntryPointsPrefix
	activityLogPrefix
	ledgerStateDiffsPrefix
)

type Prunable struct {
	Blocks           *Blocks
	SolidEntryPoints *SolidEntryPoints
	ActiveNodes      *ActiveNodes
	LedgerStateDiffs *LedgerStateDiffs
}

func New(database *database.Manager) (newPrunable *Prunable) {
	return &Prunable{
		Blocks:           NewBlocks(database, blocksPrefix),
		SolidEntryPoints: NewSolidEntryPoints(database, solidEntryPointsPrefix),
		ActiveNodes:      NewActiveNodes(database, activityLogPrefix),
		LedgerStateDiffs: NewLedgerStateDiffs(database, ledgerStateDiffsPrefix),
	}
}
