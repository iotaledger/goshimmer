package prunable

import (
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Prunable struct {
	Blocks           *Blocks
	SolidEntryPoints *SolidEntryPoints
	ActivityLog      *ActivityLog
	LedgerStateDiffs *LedgerStateDiffs
}

func New(blockStorage, stateDiffStorage, solidEntryPointsStorage, activityLogStorage func(epoch.Index) kvstore.KVStore) (newTangleStorage *Prunable) {
	return &Prunable{
		Blocks:           &Blocks{blockStorage},
		SolidEntryPoints: &SolidEntryPoints{solidEntryPointsStorage},
		ActivityLog:      &ActivityLog{activityLogStorage},
		LedgerStateDiffs: &LedgerStateDiffs{BucketedStorage: stateDiffStorage},
	}
}
