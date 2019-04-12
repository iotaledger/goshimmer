package protocol

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighborcandidates"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
    "github.com/iotaledger/goshimmer/plugins/gossip/neighbormanager"
)

func createIncomingRequestProcessor(plugin *node.Plugin) func(req *request.Request) {
    return func(req *request.Request) {
        Events.DiscoverPeer.Trigger(req.Issuer)

        if req.Issuer.Conn != nil {
            plugin.LogDebug("received TCP peering request from " + req.Issuer.String())
        } else {
            plugin.LogDebug("received UDP peering request from " + req.Issuer.String())
        }

        if len(neighbormanager.ACCEPTED_NEIGHBORS) <= constants.NEIGHBOR_COUNT / 2 {
            if err := req.Accept(proposedPeeringCandidates(req)); err != nil {
                plugin.LogDebug("error when sending response to" + req.Issuer.String())
            }

            plugin.LogDebug("sent positive peering response to " + req.Issuer.String())

            Events.IncomingRequestAccepted.Trigger(req)
        } else {
            if err := req.Reject(proposedPeeringCandidates(req)); err != nil {
                plugin.LogDebug("error when sending response to" + req.Issuer.String())
            }

            plugin.LogDebug("sent negative peering response to " + req.Issuer.String())

            Events.IncomingRequestRejected.Trigger(req)
        }
    }
}

func proposedPeeringCandidates(req *request.Request) peerlist.PeerList {
    return neighborhood.LIST_INSTANCE.Filter(func(p *peer.Peer) bool {
        return p.Identity.PublicKey != nil
    }).Sort(chosenneighborcandidates.DISTANCE(req))
}