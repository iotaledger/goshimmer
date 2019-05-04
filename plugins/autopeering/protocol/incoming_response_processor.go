package protocol

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
)

func createIncomingResponseProcessor(plugin *node.Plugin) func(peeringResponse *response.Response) {
    return func(peeringResponse *response.Response) {
        go processIncomingResponse(plugin, peeringResponse)
    }
}

func processIncomingResponse(plugin *node.Plugin, peeringResponse *response.Response) {
    plugin.LogDebug("received peering response from " + peeringResponse.Issuer.String())

    peeringResponse.Issuer.Conn.Close()

    knownpeers.INSTANCE.AddOrUpdate(peeringResponse.Issuer)
    for _, peer := range peeringResponse.Peers {
        knownpeers.INSTANCE.AddOrUpdate(peer)
    }

    if peeringResponse.Type == response.TYPE_ACCEPT {
        defer chosenneighbors.INSTANCE.Lock()()

        chosenneighbors.INSTANCE.AddOrUpdate(peeringResponse.Issuer, false)

        /*
        if len(chosenneighbors.INSTANCE.Peers) > constants.NEIGHBOR_COUNT / 2 {
            dropMessage := &drop.Drop{Issuer:ownpeer.INSTANCE}
            dropMessage.Sign()

            chosenneighbors.FurthestNeighborLock.RLock()
            if _, err := chosenneighbors.FURTHEST_NEIGHBOR.Send(dropMessage.Marshal(), types.PROTOCOL_TYPE_UDP, false); err != nil {
                plugin.LogDebug("error when sending drop message to" + chosenneighbors.FURTHEST_NEIGHBOR.String())
            }
            chosenneighbors.INSTANCE.Remove(chosenneighbors.FURTHEST_NEIGHBOR.Identity.StringIdentifier, false)
            chosenneighbors.FurthestNeighborLock.RUnlock()
        }
        */
    }
}