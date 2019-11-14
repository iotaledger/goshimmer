package acceptedneighbors

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/hive.go/events"
)

var FURTHEST_NEIGHBOR *peer.Peer

var FURTHEST_NEIGHBOR_DISTANCE = uint64(0)

var FurthestNeighborLock sync.RWMutex

func configureFurthestNeighbor() {
	INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
		FurthestNeighborLock.Lock()
		defer FurthestNeighborLock.Unlock()

		updateFurthestNeighbor(p)
	}))

	INSTANCE.Events.Remove.Attach(events.NewClosure(func(p *peer.Peer) {
		FurthestNeighborLock.Lock()
		defer FurthestNeighborLock.Unlock()

		if p.GetIdentity().StringIdentifier == FURTHEST_NEIGHBOR.GetIdentity().StringIdentifier {
			FURTHEST_NEIGHBOR_DISTANCE = uint64(0)
			FURTHEST_NEIGHBOR = nil

			for _, furthestNeighborCandidate := range INSTANCE.Peers.GetMap() {
				updateFurthestNeighbor(furthestNeighborCandidate)
			}
		}
	}))
}

func updateFurthestNeighbor(p *peer.Peer) {
	distance := OWN_DISTANCE(p)
	if distance > FURTHEST_NEIGHBOR_DISTANCE {
		FURTHEST_NEIGHBOR = p
		FURTHEST_NEIGHBOR_DISTANCE = distance
	}
}
