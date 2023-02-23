package chainmanager

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/core/identity"
)

type Fork struct {
	Source       identity.ID
	Commitment   *commitment.Commitment
	ForkingPoint *commitment.Commitment
}
