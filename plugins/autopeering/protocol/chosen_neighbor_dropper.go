package protocol

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/drop"
)

func createChosenNeighborDropper(plugin *node.Plugin) func() {
	return func() {
		timeutil.Ticker(func() {
			if len(chosenneighbors.INSTANCE.Peers) > constants.NEIGHBOR_COUNT/2 {
				defer chosenneighbors.INSTANCE.Lock()()
				for len(chosenneighbors.INSTANCE.Peers) > constants.NEIGHBOR_COUNT/2 {
					chosenneighbors.FurthestNeighborLock.RLock()
					furthestNeighbor := chosenneighbors.FURTHEST_NEIGHBOR
					chosenneighbors.FurthestNeighborLock.RUnlock()

					if furthestNeighbor != nil {
						dropMessage := &drop.Drop{Issuer: ownpeer.INSTANCE}
						dropMessage.Sign()

						chosenneighbors.INSTANCE.Remove(furthestNeighbor.Identity.StringIdentifier, false)
						go func() {
							if _, err := furthestNeighbor.Send(dropMessage.Marshal(), types.PROTOCOL_TYPE_UDP, false); err != nil {
								plugin.LogDebug("error when sending drop message to" + chosenneighbors.FURTHEST_NEIGHBOR.String())
							}
						}()
					}
				}
			}
		}, 1*time.Second)
	}
}
