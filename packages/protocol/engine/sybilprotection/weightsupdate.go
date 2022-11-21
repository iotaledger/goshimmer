package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type WeightUpdates interface {
	ApplyDiff(id identity.ID, diff int64)
	ForEach(func(id identity.ID, diff int64))
	TargetEpoch() (targetEpoch epoch.Index)
	TotalDiff() (totalDiff int64)
}

func ApplyDiff(id identity.ID, diff int64) {
	if m[id] -= int64(diff); m[id] == 0 {
		delete(m, id)
	}
}
