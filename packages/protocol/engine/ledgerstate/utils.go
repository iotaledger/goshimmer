package ledgerstate

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

func IsApply(currentEpoch, targetEpoch epoch.Index) bool {
	return currentEpoch < targetEpoch
}

func IsRollback(currentEpoch, targetEpoch epoch.Index) bool {
	return currentEpoch > targetEpoch
}
