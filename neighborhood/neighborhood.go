package neighborhood

import (
	"github.com/wollac/autopeering/distance"
	"github.com/wollac/autopeering/peer"
)

type Neighborhood struct {
	Neighbours []peer.PeerDistance
	Size       int
}

func GetFurtherestNeighbor(nh Neighborhood) peer.PeerDistance {
	if len(nh.Neighbours) < nh.Size {
		return peer.PeerDistance{
			Remote:   nil,
			Distance: distance.Max,
		}
	}
	furtherest := nh.Neighbours[0]
	for _, neighbor := range nh.Neighbours {
		if neighbor.Distance > furtherest.Distance {
			furtherest = neighbor
		}
	}
	return furtherest
}

func (nh Neighborhood) Select(candidates []peer.PeerDistance) *peer.Peer {
	target := GetFurtherestNeighbor(nh)
	for _, candidate := range candidates {
		if candidate.Distance < target.Distance {
			return candidate.Remote
		}
	}
	return nil
}
