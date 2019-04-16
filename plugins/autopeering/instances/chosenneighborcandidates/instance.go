package chosenneighborcandidates

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
    "hash/fnv"
)

var INSTANCE peerlist.PeerList

var DISTANCE = func(anchor *peer.Peer) func(p *peer.Peer) uint64 {
    return func(p *peer.Peer) uint64 {
        return hash(anchor.Identity.Identifier) ^ hash(p.Identity.Identifier)
    }
}

func init() {
    updateNeighborCandidates()

    neighborhood.Events.Update.Attach(updateNeighborCandidates)
}

func updateNeighborCandidates() {
    INSTANCE = neighborhood.LIST_INSTANCE.Sort(DISTANCE(outgoingrequest.INSTANCE.Issuer))
}

func hash(data []byte) uint64 {
    h := fnv.New64a()
    h.Write(data)

    return h.Sum64()
}

