package peer

import (
	"sort"

	"github.com/iotaledger/goshimmer/packages/autopeering/distance"
)

// PeerDistance defines the relative distance wrt a remote peer
type PeerDistance struct {
	Remote   *Peer
	Distance uint32
}

// byDistance is a slice of PeerDistance used to sort
type byDistance []PeerDistance

func (a byDistance) Len() int           { return len(a) }
func (a byDistance) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byDistance) Less(i, j int) bool { return a[i].Distance < a[j].Distance }

// NewPeerDistance returns a new PeerDistance
func NewPeerDistance(anchorID, salt []byte, remote *Peer) PeerDistance {
	return PeerDistance{
		Remote:   remote,
		Distance: distance.BySalt(anchorID, remote.ID().Bytes(), salt),
	}
}

// SortBySalt returns a slice of PeerDistance given a list of remote peers
func SortBySalt(anchor, salt []byte, remotePeers []*Peer) (result []PeerDistance) {
	result = make(byDistance, len(remotePeers))
	for i, remote := range remotePeers {
		result[i] = NewPeerDistance(anchor, salt, remote)
	}
	sort.Sort(byDistance(result))
	return result
}
