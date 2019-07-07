package protocol

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
	"github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
)

func createOutgoingRequestProcessor(plugin *node.Plugin) func() {
	return func() {
		plugin.LogInfo("Starting Chosen Neighbor Processor ...")
		plugin.LogSuccess("Starting Chosen Neighbor Processor ... done")

		sendOutgoingRequests(plugin)

		ticker := time.NewTicker(constants.FIND_NEIGHBOR_INTERVAL)
	ticker:
		for {
			select {
			case <-daemon.ShutdownSignal:
				plugin.LogInfo("Stopping Chosen Neighbor Processor ...")

				break ticker
			case <-ticker.C:
				sendOutgoingRequests(plugin)
			}
		}

		plugin.LogSuccess("Stopping Chosen Neighbor Processor ... done")
	}
}

func sendOutgoingRequests(plugin *node.Plugin) {
	for _, chosenNeighborCandidate := range chosenneighbors.CANDIDATES.Clone() {
		select {
		case <-daemon.ShutdownSignal:
			return
		case <-time.After(5 * time.Second):
		}

		if candidateShouldBeContacted(chosenNeighborCandidate) {
			if dialed, err := chosenNeighborCandidate.Send(outgoingrequest.INSTANCE.Marshal(), types.PROTOCOL_TYPE_TCP, true); err != nil {
				plugin.LogDebug(err.Error())
			} else {
				plugin.LogDebug("sent peering request to " + chosenNeighborCandidate.String())

				if dialed {
					tcp.HandleConnection(chosenNeighborCandidate.Conn)
				}
			}
		}
	}
}

func candidateShouldBeContacted(candidate *peer.Peer) bool {
	nodeId := candidate.Identity.StringIdentifier

	return (!acceptedneighbors.INSTANCE.Contains(nodeId) && !chosenneighbors.INSTANCE.Contains(nodeId) &&
		accountability.OwnId().StringIdentifier != nodeId) && (len(chosenneighbors.INSTANCE.Peers) < constants.NEIGHBOR_COUNT/2 ||
		chosenneighbors.OWN_DISTANCE(candidate) < chosenneighbors.FURTHEST_NEIGHBOR_DISTANCE)
}
