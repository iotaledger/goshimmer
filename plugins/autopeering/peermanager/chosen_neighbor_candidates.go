package peermanager

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager/types"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
)

var CHOSEN_NEIGHBOR_CANDIDATES types.PeerList

var CHOSEN_NEIGHBOR_DISTANCE = func(req *request.Request) func(p *peer.Peer) float64 {
    return func(p *peer.Peer) float64 {
        return 1
    }
}

func init() {
    updateNeighborCandidates()

    Events.UpdateNeighborhood.Attach(updateNeighborCandidates)
}

func updateNeighborCandidates() {
    CHOSEN_NEIGHBOR_CANDIDATES = NEIGHBORHOOD.List().Sort(CHOSEN_NEIGHBOR_DISTANCE(request.OUTGOING_REQUEST))
}
