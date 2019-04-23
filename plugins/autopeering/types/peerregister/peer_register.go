package peerregister

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
    "sync"
)

type PeerRegister struct {
    Peers map[string]*peer.Peer
    Lock sync.RWMutex
}

func New() *PeerRegister {
    return &PeerRegister{
        Peers: make(map[string]*peer.Peer),
    }
}

// returns true if a new entry was added
func (this *PeerRegister) AddOrUpdate(peer *peer.Peer) bool {
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

func (this *PeerRegister) Contains(key string) bool {
    if _, exists := this.Peers[key]; exists {
        return true
    } else {
        return false
    }
}

func (this *PeerRegister) Filter(filterFn func(this *PeerRegister, req *request.Request) *PeerRegister, req *request.Request) *PeerRegister {
    return filterFn(this, req)
}

func (this *PeerRegister) List() peerlist.PeerList {
    peerList := make(peerlist.PeerList, len(this.Peers))

    counter := 0
    for _, currentPeer := range this.Peers {
        peerList[counter] = currentPeer
        counter++
    }

    return peerList
}
