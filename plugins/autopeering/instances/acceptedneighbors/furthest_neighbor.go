package acceptedneighbors

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "sync"
)

var FURTHEST_NEIGHBOR *peer.Peer

var FURTHEST_NEIGHBOR_DISTANCE = uint64(0)

var FurthestNeighborLock sync.RWMutex

func configureFurthestNeighbor() {
    INSTANCE.Events.Add.Attach(func(p *peer.Peer) {
        FurthestNeighborLock.Lock()
        defer FurthestNeighborLock.Unlock()

        updateFurthestNeighbor(p)
    })

    INSTANCE.Events.Remove.Attach(func(p *peer.Peer) {
        FurthestNeighborLock.Lock()
        defer FurthestNeighborLock.Unlock()

        if p.Identity.StringIdentifier == FURTHEST_NEIGHBOR.Identity.StringIdentifier {
            FURTHEST_NEIGHBOR_DISTANCE = uint64(0)
            FURTHEST_NEIGHBOR = nil

            for _, furthestNeighborCandidate := range INSTANCE.Peers {
                updateFurthestNeighbor(furthestNeighborCandidate)
            }
        }
    })
}

func updateFurthestNeighbor(p *peer.Peer) {
    distance := OWN_DISTANCE(p)
    if distance > FURTHEST_NEIGHBOR_DISTANCE {
        FURTHEST_NEIGHBOR = p
        FURTHEST_NEIGHBOR_DISTANCE = distance
    }
}
