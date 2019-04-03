package peermanager

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
)

type PeerList struct {
    Peers map[string]*peer.Peer
}

func (this *PeerList) Update(peer *peer.Peer) bool {
    if peer.Identity == nil || bytes.Equal(peer.Identity.Identifier, accountability.OWN_ID.Identifier) {
        return false
    }

    if existingPeer, exists := this.Peers[peer.Identity.StringIdentifier]; exists {
        existingPeer.Address = peer.Address
        existingPeer.GossipPort = peer.GossipPort
        existingPeer.PeeringPort = peer.PeeringPort

        // trigger update peer

        return false
    } else {
        this.Peers[peer.Identity.StringIdentifier] = peer

        // trigger add peer

        return true
    }
}

func (this *PeerList) Add(peer *peer.Peer) {
    this.Peers[peer.Identity.StringIdentifier] = peer
}

func (this *PeerList) Contains(key string) bool {
    if _, exists := this.Peers[key]; exists {
        return true
    } else {
        return false
    }
}
