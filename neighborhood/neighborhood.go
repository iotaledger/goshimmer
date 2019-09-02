package neighborhood

import (
	"github.com/wollac/autopeering/distance"
	"github.com/wollac/autopeering/peer"
)

type Neighborhood struct {
	Neighbors []peer.PeerDistance
	Size      int
}

func (nh Neighborhood) getFurtherest() peer.PeerDistance {
	if len(nh.Neighbors) < nh.Size {
		return peer.PeerDistance{
			Remote:   nil,
			Distance: distance.Max,
		}
	}
	furtherest := nh.Neighbors[0]
	for _, neighbor := range nh.Neighbors {
		if neighbor.Distance > furtherest.Distance {
			furtherest = neighbor
		}
	}
	return furtherest
}

func (nh Neighborhood) Select(candidates []peer.PeerDistance) *peer.Peer {
	target := nh.getFurtherest()
	for _, candidate := range candidates {
		if candidate.Distance < target.Distance {
			return candidate.Remote
		}
	}
	return nil
}
