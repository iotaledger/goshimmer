package peermanager

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "time"
)

type PeerList struct {
    Peers map[string]*protocol.Peer
}

func (this *PeerList) Update(peer *protocol.Peer) {
    if peer.Identity == UNKNOWN_IDENTITY || bytes.Equal(peer.Identity.Identifier, accountability.OWN_ID.Identifier) {
        return
    }

    now := time.Now()

    if existingPeer, exists := this.Peers[peer.Identity.StringIdentifier]; exists {
        existingPeer.Address = peer.Address
        existingPeer.GossipPort = peer.GossipPort
        existingPeer.PeeringPort = peer.PeeringPort
        existingPeer.LastSeen = now
        existingPeer.LastContact = now

        // trigger update peer
    } else {
        peer.FirstSeen = now
        peer.LastSeen = now
        peer.LastContact = now

        this.Peers[peer.Identity.StringIdentifier] = peer

        // trigger add peer
    }
}

func (this *PeerList) Add(peer *protocol.Peer) {
    this.Peers[peer.Identity.StringIdentifier] = peer
}

func (this *PeerList) Contains(key string) bool {
    if _, exists := this.Peers[key]; exists {
        return true
    } else {
        return false
    }
}
