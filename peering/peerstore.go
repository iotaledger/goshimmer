package peering

import (
	"log"
	"net"
	"sync"
	"time"

	id "github.com/wollac/autopeering/identity"
)

type Peer struct {
	identity *id.Identity
	ip       net.IP
	port     uint16
	lastPong time.Time
}

func NewPeer(identity *id.Identity, ip string, port uint16) *Peer {
	return &Peer{
		identity: identity,
		ip:       net.ParseIP(ip),
		port:     port,
		lastPong: time.Now(),
	}
}

type PeerStore struct {
	peers map[string]*Peer

	mu sync.RWMutex
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[string]*Peer),
	}
}

func (ps *PeerStore) Add(peer *Peer) {
	log.Println("peerstore: adding new peer:" + peer.identity.StringId)

	ps.mu.Lock()
	ps.peers[peer.identity.StringId] = peer
	ps.mu.Unlock()
}

func (ps *PeerStore) Get(identity *id.Identity) (peer *Peer, ok bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, ok = ps.peers[identity.StringId]
	return
}

func (ps *PeerStore) GetOlder(threshold time.Duration) []*Peer {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	now := time.Now()

	result := make([]*Peer, len(ps.peers))
	for _, peer := range ps.peers {
		if now.Sub(peer.lastPong) > threshold {
			result = append(result, peer)
		}
	}

	return result
}
