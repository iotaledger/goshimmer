package protocol

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighborcandidates"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
)

func createIncomingRequestProcessor(plugin *node.Plugin) func(req *request.Request) {
    return func(req *request.Request) {
        go processIncomingRequest(plugin, req)
    }
}

func processIncomingRequest(plugin *node.Plugin, req *request.Request) {
    plugin.LogDebug("received peering request from " + req.Issuer.String())

    Events.DiscoverPeer.Trigger(req.Issuer)

    if requestingNodeIsCloser(req) {
        acceptedneighbors.INSTANCE.Lock.Lock()
        defer acceptedneighbors.INSTANCE.Lock.Unlock()

        if requestingNodeIsCloser(req) {
            acceptedneighbors.INSTANCE.AddOrUpdate(req.Issuer)
            if len(acceptedneighbors.INSTANCE.Peers) > constants.NEIGHBOR_COUNT / 2 {
                // drop further away node
            }

            acceptRequest(plugin, req)

            return
        }
    }

    rejectRequest(plugin, req)
}

func requestingNodeIsCloser(req *request.Request) bool {
    //distanceFn := chosenneighborcandidates.DISTANCE(ownpeer.INSTANCE)

    return len(acceptedneighbors.INSTANCE.Peers) <= constants.NEIGHBOR_COUNT / 2 ||
        acceptedneighbors.INSTANCE.Contains(req.Issuer.Identity.StringIdentifier)
}

func acceptRequest(plugin *node.Plugin, req *request.Request) {
    if err := req.Accept(generateProposedPeeringCandidates(req)); err != nil {
        plugin.LogDebug("error when sending response to" + req.Issuer.String())
    }

    plugin.LogDebug("sent positive peering response to " + req.Issuer.String())

    Events.IncomingRequestAccepted.Trigger(req)
}

func rejectRequest(plugin *node.Plugin, req *request.Request) {
    if err := req.Reject(generateProposedPeeringCandidates(req)); err != nil {
        plugin.LogDebug("error when sending response to" + req.Issuer.String())
    }

    plugin.LogDebug("sent negative peering response to " + req.Issuer.String())

    Events.IncomingRequestRejected.Trigger(req)
}

func generateProposedPeeringCandidates(req *request.Request) peerlist.PeerList {
    return neighborhood.LIST_INSTANCE.Filter(func(p *peer.Peer) bool {
        return p.Identity.PublicKey != nil
    }).Sort(chosenneighborcandidates.DISTANCE(req.Issuer))
}