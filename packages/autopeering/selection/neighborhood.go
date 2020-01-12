package selection

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/autopeering/distance"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
)

type Neighborhood struct {
	Neighbors []peer.PeerDistance
	Size      int
	mutex     sync.RWMutex
}

func (nh *Neighborhood) getFurthest() (peer.PeerDistance, int) {
	nh.mutex.RLock()
	defer nh.mutex.RUnlock()
	if len(nh.Neighbors) < nh.Size {
		return peer.PeerDistance{
			Remote:   nil,
			Distance: distance.Max,
		}, len(nh.Neighbors)
	}

	index := 0
	furthest := nh.Neighbors[index]
	for i, n := range nh.Neighbors {
		if n.Distance > furthest.Distance {
			furthest = n
			index = i
		}
	}
	return furthest, index
}

func (nh *Neighborhood) Select(candidates []peer.PeerDistance) peer.PeerDistance {
	if len(candidates) > 0 {
		target, _ := nh.getFurthest()
		for _, candidate := range candidates {
			if candidate.Distance < target.Distance {
				return candidate
			}
		}
	}
	return peer.PeerDistance{}
}

func (nh *Neighborhood) Add(toAdd peer.PeerDistance) {
	nh.mutex.Lock()
	defer nh.mutex.Unlock()
	if len(nh.Neighbors) < nh.Size {
		nh.Neighbors = append(nh.Neighbors, toAdd)
	}
}

// RemovePeer removes the peer with the given ID from the neighborhood.
// It returns the peer that was removed or nil of no such peer exists.
func (nh *Neighborhood) RemovePeer(id peer.ID) *peer.Peer {
	nh.mutex.Lock()
	defer nh.mutex.Unlock()

	index := nh.getPeerIndex(id)
	if index < 0 {
		return nil
	}
	n := nh.Neighbors[index]

	// remove index from slice
	if index < len(nh.Neighbors)-1 {
		copy(nh.Neighbors[index:], nh.Neighbors[index+1:])
	}
	nh.Neighbors[len(nh.Neighbors)-1] = peer.PeerDistance{}
	nh.Neighbors = nh.Neighbors[:len(nh.Neighbors)-1]

	return n.Remote
}

func (nh *Neighborhood) getPeerIndex(id peer.ID) int {
	for i, p := range nh.Neighbors {
		if p.Remote.ID() == id {
			return i
		}
	}
	return -1
}

func (nh *Neighborhood) UpdateDistance(anchor, salt []byte) {
	nh.mutex.Lock()
	defer nh.mutex.Unlock()
	for i, n := range nh.Neighbors {
		nh.Neighbors[i].Distance = distance.BySalt(anchor, n.Remote.ID().Bytes(), salt)
	}
}

func (nh *Neighborhood) GetPeers() []*peer.Peer {
	nh.mutex.RLock()
	defer nh.mutex.RUnlock()
	list := make([]*peer.Peer, len(nh.Neighbors))
	for i, n := range nh.Neighbors {
		list[i] = n.Remote
	}
	return list
}
