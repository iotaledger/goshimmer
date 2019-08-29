package peer

import (
	"github.com/wollac/autopeering/distance"
)

// List is a slice of peer
type List = []*Peer

// PeerDistance defines the relative distance wrt a remote peer
type PeerDistance struct {
	Remote   *Peer
	Distance uint32
}

// ByDistance is a slice of PeerDistance used to sort
type ByDistance []PeerDistance

func (a ByDistance) Len() int           { return len(a) }
func (a ByDistance) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByDistance) Less(i, j int) bool { return a[i].Distance < a[j].Distance }

// NewPeerDistance returns a new PeerDistance
func NewPeerDistance(anchorID, salt []byte, remote *Peer) PeerDistance {
	return PeerDistance{
		Remote:   remote,
		Distance: distance.BySalt(anchorID, remote.Identity.ID(), salt),
	}
}

// DistanceList returns a slice of PeerDistance given a list of remote peers
// which can be sorted: sort.Sort(ByDistance(result))
func DistanceList(anchor, salt []byte, remotePeers List) (result []PeerDistance) {
	result = make(ByDistance, len(remotePeers))
	for i, remote := range remotePeers {
		result[i] = NewPeerDistance(anchor, salt, remote)
	}
	return result
}
