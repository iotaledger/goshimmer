package peerregister

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
)

type PeerRegister map[string]*peer.Peer

// returns true if a new entry was added
func (this PeerRegister) AddOrUpdate(peer *peer.Peer) bool {
    if peer.Identity == nil || bytes.Equal(peer.Identity.Identifier, accountability.OWN_ID.Identifier) {
        return false
    }

    if existingPeer, exists := this[peer.Identity.StringIdentifier]; exists {
        existingPeer.Address = peer.Address
        existingPeer.GossipPort = peer.GossipPort
        existingPeer.PeeringPort = peer.PeeringPort

        // trigger update peer

        return false
    } else {
        this[peer.Identity.StringIdentifier] = peer

        // trigger add peer

        return true
    }
}

func (this PeerRegister) Contains(key string) bool {
    if _, exists := this[key]; exists {
        return true
    } else {
        return false
    }
}

func (this PeerRegister) Filter(filterFn func(this PeerRegister, req *request.Request) PeerRegister, req *request.Request) PeerRegister {
    return filterFn(this, req)
}

func (this PeerRegister) List() peerlist.PeerList {
    peerList := make(peerlist.PeerList, len(this))

    counter := 0
    for _, currentPeer := range this {
        peerList[counter] = currentPeer
        counter++
    }

    return peerList
}
