package prunable

import (
	"github.com/iotaledger/hive.go/core/generics/lo"

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
	ActivityLog      *ActivityLog
	LedgerStateDiffs *LedgerStateDiffs
}

func New(database *database.Manager) (newPrunable *Prunable) {
	return &Prunable{
		Blocks:           &Blocks{lo.Bind([]byte{blocksPrefix}, database.Get)},
		SolidEntryPoints: &SolidEntryPoints{lo.Bind([]byte{solidEntryPointsPrefix}, database.Get)},
		ActivityLog:      &ActivityLog{lo.Bind([]byte{activityLogPrefix}, database.Get)},
		LedgerStateDiffs: &LedgerStateDiffs{BucketedStorage: lo.Bind([]byte{ledgerStateDiffsPrefix}, database.Get)},
	}
}
