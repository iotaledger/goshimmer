package protocol

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
	"github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"

	"github.com/iotaledger/goshimmer/packages/timeutil"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
)

func createOutgoingRequestProcessor(plugin *node.Plugin) func() {
	return func() {
		log.Info("Starting Chosen Neighbor Processor ...")
		log.Info("Starting Chosen Neighbor Processor ... done")

		sendOutgoingRequests(plugin)

		ticker := time.NewTicker(constants.FIND_NEIGHBOR_INTERVAL)
	ticker:
		for {
			select {
			case <-daemon.ShutdownSignal:
				log.Info("Stopping Chosen Neighbor Processor ...")

				break ticker
			case <-ticker.C:
				sendOutgoingRequests(plugin)
			}
		}

		log.Info("Stopping Chosen Neighbor Processor ... done")
	}
}

func sendOutgoingRequests(plugin *node.Plugin) {
	for _, chosenNeighborCandidate := range chosenneighbors.CANDIDATES.GetPeers() {
		timeutil.Sleep(5 * time.Second)

		if candidateShouldBeContacted(chosenNeighborCandidate) {
			doneChan := make(chan int, 1)

			go func(doneChan chan int) {
				if dialed, err := chosenNeighborCandidate.Send(outgoingrequest.INSTANCE.Marshal(), types.PROTOCOL_TYPE_TCP, true); err != nil {
					log.Debug(err.Error())
				} else {
					log.Debugf("sent peering request to %s", chosenNeighborCandidate.String())

					if dialed {
						tcp.HandleConnection(chosenNeighborCandidate.GetConn())
					}
				}

				close(doneChan)
			}(doneChan)

			select {
			case <-daemon.ShutdownSignal:
				return
			case <-doneChan:
				continue
			}
		}
	}
}

func candidateShouldBeContacted(candidate *peer.Peer) bool {
	nodeId := candidate.GetIdentity().StringIdentifier

	chosenneighbors.FurthestNeighborLock.RLock()
	defer chosenneighbors.FurthestNeighborLock.RUnlock()

	return (!acceptedneighbors.INSTANCE.Contains(nodeId) && !chosenneighbors.INSTANCE.Contains(nodeId) &&
		accountability.OwnId().StringIdentifier != nodeId) && (chosenneighbors.INSTANCE.Peers.Len() < constants.NEIGHBOR_COUNT/2 ||
		chosenneighbors.OWN_DISTANCE(candidate) < chosenneighbors.FURTHEST_NEIGHBOR_DISTANCE)
}
