package protocol

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"
    "github.com/iotaledger/goshimmer/plugins/gossip/neighbormanager"
    "time"
)

func createChosenNeighborProcessor(plugin *node.Plugin) func() {
    return func() {
        plugin.LogInfo("Starting Chosen Neighbor Processor ...")
        plugin.LogSuccess("Starting Chosen Neighbor Processor ... done")

        chooseNeighbors(plugin)

        ticker := time.NewTicker(constants.FIND_NEIGHBOR_INTERVAL)
        ticker:
        for {
            select {
            case <- daemon.ShutdownSignal:
                plugin.LogInfo("Stopping Chosen Neighbor Processor ...")

                break ticker
            case <- ticker.C:
                chooseNeighbors(plugin)
            }
        }

        plugin.LogSuccess("Stopping Chosen Neighbor Processor ... done")
    }
}

func chooseNeighbors(plugin *node.Plugin) {
    for _, chosenNeighborCandidate := range peermanager.CHOSEN_NEIGHBOR_CANDIDATES {
        go func(peer *peer.Peer) {
            nodeId := peer.Identity.StringIdentifier

            if !neighbormanager.ACCEPTED_NEIGHBORS.Contains(nodeId) &&
                !neighbormanager.CHOSEN_NEIGHBORS.Contains(nodeId) &&
                accountability.OWN_ID.StringIdentifier != nodeId {

                if connectionAlive, err := request.Send(peer); err != nil {
                    plugin.LogDebug(err.Error())
                } else if connectionAlive {
                    plugin.LogDebug("sent TCP peering request to " + peer.String())

                    tcp.HandleConnection(peer.Conn)
                } else {
                    plugin.LogDebug("sent UDP peering request to " + peer.String())
                }
            }
        }(chosenNeighborCandidate)
    }
}
