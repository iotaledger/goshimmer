package state

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type UpdateConsumer interface {
	CreateBatchedStateTransition(targetEpoch epoch.Index) BatchedTransition
	LastConsumedEpoch() epoch.Index
}
