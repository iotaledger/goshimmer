package protocol

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighborcandidates"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"
    "github.com/iotaledger/goshimmer/plugins/gossip/neighbormanager"
    "time"
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
            case <- daemon.ShutdownSignal:
                plugin.LogInfo("Stopping Chosen Neighbor Processor ...")

                break ticker
            case <- ticker.C:
                sendOutgoingRequests(plugin)
            }
        }

        plugin.LogSuccess("Stopping Chosen Neighbor Processor ... done")
    }
}

func sendOutgoingRequests(plugin *node.Plugin) {
    for _, chosenNeighborCandidate := range chosenneighborcandidates.INSTANCE {
        go func(peer *peer.Peer) {
            nodeId := peer.Identity.StringIdentifier

            if !neighbormanager.ACCEPTED_NEIGHBORS.Contains(nodeId) &&
                !neighbormanager.CHOSEN_NEIGHBORS.Contains(nodeId) &&
                accountability.OWN_ID.StringIdentifier != nodeId {

                if dialed, err := peer.Send(outgoingrequest.INSTANCE.Marshal(), types.PROTOCOL_TYPE_TCP, true); err != nil {
                    plugin.LogDebug(err.Error())
                } else {
                    plugin.LogDebug("sent peering request to " + peer.String())

                    if dialed {
                        tcp.HandleConnection(peer.Conn)
                    }
                }
            }
        }(chosenNeighborCandidate)
    }
}
