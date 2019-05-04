package chosenneighbors

import (
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "sync"
)

var FURTHEST_NEIGHBOR *peer.Peer

var FURTHEST_NEIGHBOR_DISTANCE = uint64(0)

var FurthestNeighborLock sync.RWMutex

func configureFurthestNeighbor() {
    INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
        FurthestNeighborLock.Lock()
        defer FurthestNeighborLock.Unlock()

        distance := OWN_DISTANCE(p)
        if distance > FURTHEST_NEIGHBOR_DISTANCE {
            FURTHEST_NEIGHBOR = p
            FURTHEST_NEIGHBOR_DISTANCE = distance
        }
    }))

    INSTANCE.Events.Remove.Attach(events.NewClosure(func(p *peer.Peer) {
        FurthestNeighborLock.Lock()
        defer FurthestNeighborLock.Unlock()

        if p == FURTHEST_NEIGHBOR {
            FURTHEST_NEIGHBOR_DISTANCE = uint64(0)
            FURTHEST_NEIGHBOR = nil

            for _, furthestNeighborCandidate := range INSTANCE.Peers {
                distance := OWN_DISTANCE(furthestNeighborCandidate)
                if distance > FURTHEST_NEIGHBOR_DISTANCE {
                    FURTHEST_NEIGHBOR = furthestNeighborCandidate
                    FURTHEST_NEIGHBOR_DISTANCE = distance
                }
            }
        }
    }))
}