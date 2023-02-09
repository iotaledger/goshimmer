package chainmanager

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Fork struct {
	Source       identity.ID
	Commitment   *commitment.Commitment
	ForkingPoint *commitment.Commitment
}
