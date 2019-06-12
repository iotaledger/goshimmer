package peerregister

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
)

type PeerRegister struct {
	Peers  map[string]*peer.Peer
	Events peerRegisterEvents
	Lock   sync.RWMutex
}

func New() *PeerRegister {
	return &PeerRegister{
		Peers: make(map[string]*peer.Peer),
		Events: peerRegisterEvents{
			Add:    events.NewEvent(peerCaller),
			Update: events.NewEvent(peerCaller),
			Remove: events.NewEvent(peerCaller),
		},
	}
}

// returns true if a new entry was added
func (this *PeerRegister) AddOrUpdate(peer *peer.Peer) bool {
	this.Lock.Lock()
	defer this.Lock.Unlock()

	if peer.Identity == nil || bytes.Equal(peer.Identity.Identifier, accountability.OwnId().Identifier) {
		return false
	}

	if existingPeer, exists := this.Peers[peer.Identity.StringIdentifier]; exists {
		existingPeer.Address = peer.Address
		existingPeer.GossipPort = peer.GossipPort
		existingPeer.PeeringPort = peer.PeeringPort

		this.Events.Update.Trigger(existingPeer)

		return false
	} else {
		this.Peers[peer.Identity.StringIdentifier] = peer

		this.Events.Add.Trigger(peer)

		return true
	}
}

// by calling defer peerRegister.Lock()() we can auto-lock AND unlock (note: two parentheses)
// func (this *PeerRegister) Lock() func() {
// 	this.Lock.Lock()

// 	return this.Lock.Unlock
// }

func (this *PeerRegister) Remove(key string, lock ...bool) {
	this.Lock.Lock()
	defer this.Lock.Unlock()
	if peerEntry, exists := this.Peers[key]; exists {
		delete(this.Peers, key)
		this.Events.Remove.Trigger(peerEntry)
	}
}

func (this *PeerRegister) Contains(key string) bool {
	this.Lock.Lock()
	defer this.Lock.Unlock()
	if _, exists := this.Peers[key]; exists {
		return true
	} else {
		return false
	}
}

func (this *PeerRegister) Filter(filterFn func(this *PeerRegister, req *request.Request) *PeerRegister, req *request.Request) *PeerRegister {
	this.Lock.Lock()
	defer this.Lock.Unlock()
	return filterFn(this, req)
}

func (this *PeerRegister) List() peerlist.PeerList {
	this.Lock.Lock()
	defer this.Lock.Unlock()
	peerList := make(peerlist.PeerList, len(this.Peers))

	counter := 0
	for _, currentPeer := range this.Peers {
		peerList[counter] = currentPeer
		counter++
	}

	return peerList
}
