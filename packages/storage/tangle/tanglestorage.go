package tangle

import (
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Tangle struct {
	BlockStorage            *BlockStorage
	SolidEntryPointsStorage *SolidEntryPointsStorage
	ActivityLogStorage      *ActivityLogStorage
}

func New(blockStorage, solidEntryPointsStorage, activityLogStorage func(epoch.Index) kvstore.KVStore) (newTangleStorage *Tangle) {
	return &Tangle{
		BlockStorage:            &BlockStorage{blockStorage},
		SolidEntryPointsStorage: &SolidEntryPointsStorage{solidEntryPointsStorage},
		ActivityLogStorage:      &ActivityLogStorage{activityLogStorage},
	}
}
