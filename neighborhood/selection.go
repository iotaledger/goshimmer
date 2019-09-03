package neighborhood

import (
	"github.com/wollac/autopeering/peer"
)

type Selector interface {
	Select(candidates peer.PeerDistance) *peer.Peer
}

type Filter map[*peer.Peer]bool

func (f Filter) Apply(list []peer.PeerDistance) (filteredList []peer.PeerDistance) {
	for _, peer := range list {
		if !f[peer.Remote] {
			filteredList = append(filteredList, peer)
		}
	}
	return filteredList
}

func (f Filter) AddNeighborhood(n Neighborhood) {
	for _, peer := range n.Neighbors {
		f[peer.Remote] = true
	}
}

func (f Filter) AddPeer(peer *peer.Peer) {
	f[peer] = true
}

func (f Filter) RemovePeer(peer *peer.Peer) {
	f[peer] = false
}

func (f Filter) JoinFilter(x Filter) {
	for k, v := range x {
		f[k] = v
	}
}
