package chosenneighborcandidates

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
)

var INSTANCE peerlist.PeerList

var DISTANCE = func(req *request.Request) func(p *peer.Peer) float64 {
    return func(p *peer.Peer) float64 {
        return 1
    }
}

func init() {
    updateNeighborCandidates()

    neighborhood.Events.Update.Attach(updateNeighborCandidates)
}

func updateNeighborCandidates() {
    INSTANCE = neighborhood.LIST_INSTANCE.Sort(DISTANCE(outgoingrequest.INSTANCE))
}

