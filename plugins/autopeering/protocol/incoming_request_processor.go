package protocol

import (
	"math/rand"

	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
	"github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/parameter"
)

func createIncomingRequestProcessor(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(req *request.Request) {
		go processIncomingRequest(plugin, req)
	})
}

func processIncomingRequest(plugin *node.Plugin, req *request.Request) {
	log.Debugf("received peering request from %s", req.Issuer.String())

	knownpeers.INSTANCE.AddOrUpdate(req.Issuer)

	if parameter.NodeConfig.GetBool(parameters.CFG_ACCEPT_REQUESTS) && requestShouldBeAccepted(req) {
		defer acceptedneighbors.INSTANCE.Lock()()

		if requestShouldBeAccepted(req) {
			acceptedneighbors.INSTANCE.AddOrUpdate(req.Issuer)

			acceptRequest(plugin, req)

			return
		}
	}

	rejectRequest(plugin, req)
}

func requestShouldBeAccepted(req *request.Request) bool {
	return acceptedneighbors.INSTANCE.Peers.Len() < constants.NEIGHBOR_COUNT/2 ||
		acceptedneighbors.INSTANCE.Contains(req.Issuer.GetIdentity().StringIdentifier) ||
		acceptedneighbors.OWN_DISTANCE(req.Issuer) < acceptedneighbors.FURTHEST_NEIGHBOR_DISTANCE
}

func acceptRequest(plugin *node.Plugin, req *request.Request) {
	if err := req.Accept(generateProposedPeeringCandidates(req).GetPeers()); err != nil {
		log.Debugf("error when sending response to %s", req.Issuer.String())
	}

	log.Debugf("sent positive peering response to %s", req.Issuer.String())

	acceptedneighbors.INSTANCE.AddOrUpdate(req.Issuer)
}

func rejectRequest(plugin *node.Plugin, req *request.Request) {
	if err := req.Reject(generateProposedPeeringCandidates(req).GetPeers()); err != nil {
		log.Debugf("error when sending response to %s", req.Issuer.String())
	}

	log.Debugf("sent negative peering response to %s", req.Issuer.String())
}

func generateProposedPeeringCandidates(req *request.Request) *peerlist.PeerList {
	proposedPeers := neighborhood.LIST_INSTANCE.Filter(func(p *peer.Peer) bool {
		return p.GetIdentity().PublicKey != nil
	})
	rand.Shuffle(proposedPeers.Len(), func(i, j int) {
		proposedPeers.SwapPeers(i, j)
	})

	return proposedPeers
}
