package neighborhood

import (
	"github.com/wollac/autopeering/peer"
)

type Selector interface {
	Select(candidates peer.PeerDistance) *peer.Peer
}

type Filter map[*peer.Peer]bool

func (f Filter) Complement(list []peer.PeerDistance) (filteredList []peer.PeerDistance) {
	for _, peer := range list {
		if !f[peer.Remote] {
			filteredList = append(filteredList, peer)
		}
	}
	return filteredList
}
