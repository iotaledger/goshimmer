package selection

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
)

type Selector interface {
	Select(candidates peer.PeerDistance) *peer.Peer
}

type Filter struct {
	internal map[peer.ID]bool
	lock     sync.RWMutex
}

func NewFilter() *Filter {
	return &Filter{
		internal: make(map[peer.ID]bool),
	}
}

func (f *Filter) Apply(list []peer.PeerDistance) (filteredList []peer.PeerDistance) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	for _, peer := range list {
		if !f.internal[peer.Remote.ID()] {
			filteredList = append(filteredList, peer)
		}
	}
	return filteredList
}

func (f *Filter) PushBack(list []peer.PeerDistance) (filteredList []peer.PeerDistance) {
	var head, tail []peer.PeerDistance
	f.lock.RLock()
	defer f.lock.RUnlock()
	for _, peer := range list {
		if f.internal[peer.Remote.ID()] {
			tail = append(tail, peer)
		} else {
			head = append(head, peer)
		}
	}
	return append(head, tail...)
}

func (f *Filter) AddPeers(n []*peer.Peer) {
	f.lock.Lock()
	for _, peer := range n {
		f.internal[peer.ID()] = true
	}
	f.lock.Unlock()
}

func (f *Filter) AddPeer(peer peer.ID) {
	f.lock.Lock()
	f.internal[peer] = true
	f.lock.Unlock()
}

func (f *Filter) RemovePeer(peer peer.ID) {
	f.lock.Lock()
	f.internal[peer] = false
	f.lock.Unlock()
}

func (f *Filter) Clean() {
	f.lock.Lock()
	f.internal = make(map[peer.ID]bool)
	f.lock.Unlock()
}
