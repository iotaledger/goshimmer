package selection

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/autopeering/distance"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
)

type Neighborhood struct {
	neighbors []peer.PeerDistance
	size      int
	mu        sync.RWMutex
}

func NewNeighborhood(size int) *Neighborhood {
	return &Neighborhood{
		neighbors: []peer.PeerDistance{},
		size:      size,
	}
}

func (nh *Neighborhood) String() string {
	return fmt.Sprintf("%d/%d", nh.GetNumPeers(), nh.size)
}

func (nh *Neighborhood) getFurthest() (peer.PeerDistance, int) {
	nh.mu.RLock()
	defer nh.mu.RUnlock()
	if len(nh.neighbors) < nh.size {
		return peer.PeerDistance{
			Remote:   nil,
			Distance: distance.Max,
		}, len(nh.neighbors)
	}

	index := 0
	furthest := nh.neighbors[index]
	for i, n := range nh.neighbors {
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
	nh.mu.Lock()
	defer nh.mu.Unlock()
	if len(nh.neighbors) < nh.size {
		nh.neighbors = append(nh.neighbors, toAdd)
	}
}

// RemovePeer removes the peer with the given ID from the neighborhood.
// It returns the peer that was removed or nil of no such peer exists.
func (nh *Neighborhood) RemovePeer(id peer.ID) *peer.Peer {
	nh.mu.Lock()
	defer nh.mu.Unlock()

	index := nh.getPeerIndex(id)
	if index < 0 {
		return nil
	}
	n := nh.neighbors[index]

	// remove index from slice
	if index < len(nh.neighbors)-1 {
		copy(nh.neighbors[index:], nh.neighbors[index+1:])
	}
	nh.neighbors[len(nh.neighbors)-1] = peer.PeerDistance{}
	nh.neighbors = nh.neighbors[:len(nh.neighbors)-1]

	return n.Remote
}

func (nh *Neighborhood) getPeerIndex(id peer.ID) int {
	for i, p := range nh.neighbors {
		if p.Remote.ID() == id {
			return i
		}
	}
	return -1
}

func (nh *Neighborhood) UpdateDistance(anchor, salt []byte) {
	nh.mu.Lock()
	defer nh.mu.Unlock()
	for i, n := range nh.neighbors {
		nh.neighbors[i].Distance = distance.BySalt(anchor, n.Remote.ID().Bytes(), salt)
	}
}

func (nh *Neighborhood) IsFull() bool {
	nh.mu.RLock()
	defer nh.mu.RUnlock()
	return len(nh.neighbors) >= nh.size
}

func (nh *Neighborhood) GetPeers() []*peer.Peer {
	nh.mu.RLock()
	defer nh.mu.RUnlock()
	result := make([]*peer.Peer, len(nh.neighbors))
	for i, n := range nh.neighbors {
		result[i] = n.Remote
	}
	return result
}

func (nh *Neighborhood) GetNumPeers() int {
	nh.mu.RLock()
	defer nh.mu.RUnlock()
	return len(nh.neighbors)
}
