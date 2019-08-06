package peerlist

import (
	"sort"
	"sync"

	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
)

type PeerList struct {
	sync.RWMutex
	internal []*peer.Peer
}

func NewPeerList(list ...[]*peer.Peer) *PeerList {
	if list != nil {
		return &PeerList{
			internal: list[0],
		}
	}
	return &PeerList{
		internal: make([]*peer.Peer, 0),
	}
}

func (pl *PeerList) Len() int {
	pl.RLock()
	defer pl.RUnlock()
	return len(pl.internal)
}

func (pl *PeerList) AddPeer(peer *peer.Peer) {
	pl.Lock()
	pl.internal = append(pl.internal, peer)
	pl.Unlock()
}

func (pl *PeerList) GetPeers() (result []*peer.Peer) {
	pl.RLock()
	result = pl.internal
	pl.RUnlock()

	return
}

func (pl *PeerList) Update(list []*peer.Peer) {
	pl.Lock()
	pl.internal = list
	pl.Unlock()

	return
}

func (pl *PeerList) SwapPeers(i, j int) {
	pl.Lock()
	pl.internal[i], pl.internal[j] = pl.internal[j], pl.internal[i]
	pl.Unlock()
}

func (pl *PeerList) Clone() *PeerList {
	list := make([]*peer.Peer, pl.Len())
	pl.RLock()
	copy(list, pl.internal)
	pl.RUnlock()

	return NewPeerList(list)
}

func (pl *PeerList) Filter(predicate func(p *peer.Peer) bool) *PeerList {
	peerList := make([]*peer.Peer, pl.Len())

	counter := 0
	pl.RLock()
	for _, peer := range pl.internal {
		if predicate(peer) {
			peerList[counter] = peer
			counter++
		}
	}
	pl.RUnlock()

	return NewPeerList(peerList[:counter])
}

// Sorts the PeerRegister by their distance to an anchor.
func (pl *PeerList) Sort(distance func(p *peer.Peer) uint64) *PeerList {
	pl.Lock()
	defer pl.Unlock()
	sort.Slice(pl.internal, func(i, j int) bool {
		return distance(pl.internal[i]) < distance(pl.internal[j])
	})

	return pl
}
