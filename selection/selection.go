package selection

import (
	"sort"

	"github.com/wollac/autopeering/distance"
	"github.com/wollac/autopeering/peer"
)

type Neighbourhood []peer.PeerDistance

type Filter map[*peer.Peer]bool

func (f Filter) Complement(list []peer.PeerDistance) (filteredList []peer.PeerDistance) {
	for _, peer := range list {
		if !f[peer.Remote] {
			filteredList = append(filteredList, peer)
		}
	}
	return filteredList
}

func GetFurtherestNeighbor(nh Neighbourhood) peer.PeerDistance {
	if len(nh) < 4 {
		return peer.PeerDistance{
			Remote:   nil,
			Distance: distance.Max,
		}
	}
	sort.Sort(peer.ByDistance(nh))
	return nh[len(nh)-1]
}

func GetNextCandidate(nh Neighbourhood, candidates peer.ByDistance) *peer.Peer {
	target := GetFurtherestNeighbor(nh)
	for _, candidate := range candidates {
		if candidate.Distance < target.Distance {
			return candidate.Remote
		}
	}
	return nil
}
