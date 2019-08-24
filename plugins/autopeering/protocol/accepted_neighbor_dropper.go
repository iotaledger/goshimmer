package protocol

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/drop"
)

func createAcceptedNeighborDropper(plugin *node.Plugin) func() {
	return func() {
		timeutil.Ticker(func() {
			if acceptedneighbors.INSTANCE.Peers.Len() > constants.NEIGHBOR_COUNT/2 {
				defer acceptedneighbors.INSTANCE.Lock()()
				for acceptedneighbors.INSTANCE.Peers.Len() > constants.NEIGHBOR_COUNT/2 {
					acceptedneighbors.FurthestNeighborLock.RLock()
					furthestNeighbor := acceptedneighbors.FURTHEST_NEIGHBOR
					acceptedneighbors.FurthestNeighborLock.RUnlock()

					if furthestNeighbor != nil {
						dropMessage := &drop.Drop{Issuer: ownpeer.INSTANCE}
						dropMessage.Sign()

						acceptedneighbors.INSTANCE.Remove(furthestNeighbor.GetIdentity().StringIdentifier)
						go func() {
							if _, err := furthestNeighbor.Send(dropMessage.Marshal(), types.PROTOCOL_TYPE_UDP, false); err != nil {
								plugin.LogDebug("error when sending drop message to" + acceptedneighbors.FURTHEST_NEIGHBOR.String())
							}
						}()
					}
				}
			}
		}, 1*time.Second)
	}
}
