package protocol

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "strconv"
)

func createIncomingResponseProcessor(plugin *node.Plugin) func(peeringResponse *response.Response) {
    return func(peeringResponse *response.Response) {
        Events.DiscoverPeer.Trigger(peeringResponse.Issuer)
        for _, peer := range peeringResponse.Peers {
            Events.DiscoverPeer.Trigger(peer)
        }

        if peeringResponse.Issuer.Conn == nil {
            plugin.LogDebug("received UDP peering response from " + peeringResponse.Issuer.String())
        } else {
            plugin.LogDebug("received TCP peering response from " + peeringResponse.Issuer.String())

            peeringResponse.Issuer.Conn.Close()
        }

        switch peeringResponse.Type {
        case response.TYPE_ACCEPT:
            Events.OutgoingRequestAccepted.Trigger(peeringResponse)
        case response.TYPE_REJECT:
            Events.OutgoingRequestRejected.Trigger(peeringResponse)
        default:
            plugin.LogDebug("invalid response type in peering response of " + peeringResponse.Issuer.Address.String() + ":" + strconv.Itoa(int(peeringResponse.Issuer.PeeringPort)))
        }
    }
}
