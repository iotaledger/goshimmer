package peerregister

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
	"github.com/iotaledger/hive.go/events"
)

type PeerRegister struct {
	Peers  *PeerMap
	Events peerRegisterEvents
	lock   sync.RWMutex
}

func New() *PeerRegister {
	return &PeerRegister{
		Peers: NewPeerMap(),
		Events: peerRegisterEvents{
			Add:    events.NewEvent(peerCaller),
			Update: events.NewEvent(peerCaller),
			Remove: events.NewEvent(peerCaller),
		},
	}
}

// returns true if a new entry was added
func (this *PeerRegister) AddOrUpdate(peer *peer.Peer) bool {
	if peer.GetIdentity() == nil || bytes.Equal(peer.GetIdentity().Identifier, accountability.OwnId().Identifier) {
		return false
	}

	if existingPeer, exists := this.Peers.Load(peer.GetIdentity().StringIdentifier); exists {
		existingPeer.SetAddress(peer.GetAddress())
		existingPeer.SetGossipPort(peer.GetGossipPort())
		existingPeer.SetPeeringPort(peer.GetPeeringPort())
		existingPeer.SetSalt(peer.GetSalt())

		// also update the public key if not yet present
		if existingPeer.GetIdentity().PublicKey == nil {
			existingPeer.SetIdentity(peer.GetIdentity())
		}

		this.Events.Update.Trigger(existingPeer)

		return false
	} else {
		this.Peers.Store(peer.GetIdentity().StringIdentifier, peer)

		this.Events.Add.Trigger(peer)

		return true
	}
}

// by calling defer peerRegister.Lock()() we can auto-lock AND unlock (note: two parentheses)
func (this *PeerRegister) Lock() func() {
	this.lock.Lock()

	return this.lock.Unlock
}

func (this *PeerRegister) Remove(key string) {
	if peerEntry, exists := this.Peers.Delete(key); exists {
		this.Events.Remove.Trigger(peerEntry)
	}
}

func (this *PeerRegister) Contains(key string) bool {
	_, exists := this.Peers.Load(key)
	return exists
}

func (this *PeerRegister) Filter(filterFn func(this *PeerRegister, req *request.Request) *PeerRegister, req *request.Request) *PeerRegister {
	return filterFn(this, req)
}

// func (this *PeerRegister) List() *peerlist.PeerList {
// 	return peerlist.NewPeerList(this.Peers.GetSlice())
// }

func (this *PeerRegister) List() []*peer.Peer {
	return this.Peers.GetSlice()
}
