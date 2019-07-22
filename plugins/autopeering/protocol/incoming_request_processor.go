package protocol

import (
	"math/rand"

	"github.com/iotaledger/goshimmer/plugins/autopeering/parameters"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
)

func createIncomingRequestProcessor(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(req *request.Request) {
		go processIncomingRequest(plugin, req)
	})
}

func processIncomingRequest(plugin *node.Plugin, req *request.Request) {
	plugin.LogDebug("received peering request from " + req.Issuer.String())

	knownpeers.INSTANCE.AddOrUpdate(req.Issuer)

	if *parameters.ACCEPT_REQUESTS.Value && requestShouldBeAccepted(req) {
		defer acceptedneighbors.INSTANCE.Lock()()

		if requestShouldBeAccepted(req) {
			acceptedneighbors.INSTANCE.AddOrUpdate(req.Issuer, false)

			acceptRequest(plugin, req)

			return
		}
	}

	rejectRequest(plugin, req)
}

func requestShouldBeAccepted(req *request.Request) bool {
	return len(acceptedneighbors.INSTANCE.Peers) < constants.NEIGHBOR_COUNT/2 ||
		acceptedneighbors.INSTANCE.Contains(req.Issuer.Identity.StringIdentifier) ||
		acceptedneighbors.OWN_DISTANCE(req.Issuer) < acceptedneighbors.FURTHEST_NEIGHBOR_DISTANCE
}

func acceptRequest(plugin *node.Plugin, req *request.Request) {
	if err := req.Accept(generateProposedPeeringCandidates(req)); err != nil {
		plugin.LogDebug("error when sending response to" + req.Issuer.String())
	}

	plugin.LogDebug("sent positive peering response to " + req.Issuer.String())

	acceptedneighbors.INSTANCE.AddOrUpdate(req.Issuer, false)
}

func rejectRequest(plugin *node.Plugin, req *request.Request) {
	if err := req.Reject(generateProposedPeeringCandidates(req)); err != nil {
		plugin.LogDebug("error when sending response to" + req.Issuer.String())
	}

	plugin.LogDebug("sent negative peering response to " + req.Issuer.String())
}

func generateProposedPeeringCandidates(req *request.Request) peerlist.PeerList {
	proposedPeers := neighborhood.LIST_INSTANCE.Filter(func(p *peer.Peer) bool {
		return p.Identity.PublicKey != nil
	})
	rand.Shuffle(len(proposedPeers), func(i, j int) {
		proposedPeers[i], proposedPeers[j] = proposedPeers[j], proposedPeers[i]
	})

	return proposedPeers
}
