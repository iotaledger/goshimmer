package dispatcher

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type ChainTracker struct {
	solidChains map[epoch.EC]ChainID
}
