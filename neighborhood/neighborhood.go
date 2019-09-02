package neighborhood

import (
	"github.com/wollac/autopeering/distance"
	"github.com/wollac/autopeering/peer"
)

type Neighborhood struct {
	Neighbors []peer.PeerDistance
	Size      int
}

func (nh Neighborhood) getFurtherest() (peer.PeerDistance, int) {
	if len(nh.Neighbors) < nh.Size {
		return peer.PeerDistance{
			Remote:   nil,
			Distance: distance.Max,
		}, len(nh.Neighbors)
	}

	index := 0
	furtherest := nh.Neighbors[index]
	for i, neighbor := range nh.Neighbors {
		if neighbor.Distance > furtherest.Distance {
			furtherest = neighbor
			index = i
		}
	}
	return furtherest, index
}

func (nh Neighborhood) Select(candidates []peer.PeerDistance) peer.PeerDistance {
	target, _ := nh.getFurtherest()
	for _, candidate := range candidates {
		if candidate.Distance < target.Distance {
			return candidate
		}
	}
	return peer.PeerDistance{}
}

func (nh Neighborhood) Add(toAdd peer.PeerDistance) (toDrop *peer.Peer) {
	_, index := nh.getFurtherest()
	toDrop = nh.Neighbors[index].Remote
	nh.Neighbors[index] = toAdd
	return toDrop
}

func (nh Neighborhood) UpdateDistance(anchor, salt []byte) {
	for i, peer := range nh.Neighbors {
		nh.Neighbors[i].Distance = distance.BySalt(anchor, peer.Remote.ID().Bytes(), salt)
	}
}
