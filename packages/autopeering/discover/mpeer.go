package discover

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
)

// mpeer represents a discovered peer with additional data.
// The fields of Peer may not be modified.
type mpeer struct {
	peer.Peer

	verifiedCount uint // how often that peer has been reverified
	lastNewPeers  uint // number of returned new peers when queried the last time
}

func wrapPeer(p *peer.Peer) *mpeer {
	return &mpeer{Peer: *p}
}

func unwrapPeer(p *mpeer) *peer.Peer {
	return &p.Peer
}

func unwrapPeers(ps []*mpeer) []*peer.Peer {
	result := make([]*peer.Peer, len(ps))
	for i, n := range ps {
		result[i] = unwrapPeer(n)
	}
	return result
}

// containsPeer returns true if a peer with the given ID is in the list.
func containsPeer(list []*mpeer, id peer.ID) bool {
	for _, p := range list {
		if p.ID() == id {
			return true
		}
	}
	return false
}

// unshiftPeer adds a new peer to the front of the list.
// If the list already contains max peers, the last is discarded.
func unshiftPeer(list []*mpeer, p *mpeer, max int) []*mpeer {
	if len(list) > max {
		panic(fmt.Sprintf("mpeer: invalid max value %d", max))
	}
	if len(list) < max {
		list = append(list, nil)
	}
	copy(list[1:], list)
	list[0] = p

	return list
}

// deletePeer is a helper that deletes the peer with the given index from the list.
func deletePeer(list []*mpeer, i int) ([]*mpeer, *mpeer) {
	if i >= len(list) {
		panic("mpeer: invalid index or empty mpeer list")
	}
	p := list[i]

	copy(list[i:], list[i+1:])
	list[len(list)-1] = nil

	return list[:len(list)-1], p
}

// deletePeerByID deletes the peer with the given ID from the list.
func deletePeerByID(list []*mpeer, id peer.ID) ([]*mpeer, *mpeer) {
	for i, p := range list {
		if p.ID() == id {
			return deletePeer(list, i)
		}
	}
	panic("mpeer: id not contained in list")
}

// pushPeer adds the given peer to the pack of the list.
// If the list already contains max peers, the first is discarded.
func pushPeer(list []*mpeer, p *mpeer, max int) []*mpeer {
	if len(list) > max {
		panic(fmt.Sprintf("mpeer: invalid max value %d", max))
	}
	if len(list) == max {
		copy(list, list[1:])
		list[len(list)-1] = p
		return list
	}

	return append(list, p)
}
