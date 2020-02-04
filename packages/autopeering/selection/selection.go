package selection

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
)

type Selector interface {
	Select(candidates peer.PeerDistance) *peer.Peer
}

type Filter struct {
	internal   map[peer.ID]bool
	conditions []func(*peer.Peer) bool
	lock       sync.RWMutex
}

func NewFilter() *Filter {
	return &Filter{
		internal: make(map[peer.ID]bool),
	}
}

func (f *Filter) Apply(list []peer.PeerDistance) (filtered []peer.PeerDistance) {
	f.lock.RLock()
	defer f.lock.RUnlock()

list:
	for _, p := range list {
		if _, contains := f.internal[p.Remote.ID()]; contains {
			continue
		}
		for _, c := range f.conditions {
			if !c(p.Remote) {
				continue list
			}
		}
		filtered = append(filtered, p)
	}
	return
}

func (f *Filter) AddPeers(n []*peer.Peer) {
	f.lock.Lock()
	for _, p := range n {
		f.internal[p.ID()] = true
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

func (f *Filter) AddCondition(c func(p *peer.Peer) bool) {
	f.conditions = append(f.conditions, c)
}

func (f *Filter) Clean() {
	f.lock.Lock()
	f.internal = make(map[peer.ID]bool)
	f.lock.Unlock()
}
