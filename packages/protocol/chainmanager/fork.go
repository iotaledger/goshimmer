package chainmanager

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Fork struct {
	Source       identity.ID
	Commitment   *commitment.Commitment
	ForkingPoint *commitment.Commitment
}

func (e *Fork) EndEpoch() epoch.Index {
	return e.Commitment.Index()
}
