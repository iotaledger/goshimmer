package manualpeering

import (
	"context"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"sync"
)

type Manager struct {
	sync.RWMutex
	neighbors []*peer.Peer
	gm        *gossip.Manager
}

func NewManager(gm *gossip.Manager) *Manager {
	m := &Manager{gm: gm}
	gm.NeighborsEvents(gossip.NeighborsGroupManual).ConnectionFailed.Attach(events.NewClosure(m.onGossipConnFailure))
	gm.NeighborsEvents(gossip.NeighborsGroupManual).NeighborRemoved.Attach(events.NewClosure(m.onGossipNeighborRemoved))
	return m
}

func (m *Manager) AddNeighbor(ctx context.Context, p *peer.Peer) error {
	return nil
}

func (m *Manager) DropNeighbor(ctx context.Context, key ed25519.PublicKey) error {
	return nil
}

func (m *Manager) onGossipConnFailure(p *peer.Peer, connErr error) {

}

func (m *Manager) onGossipNeighborRemoved(n *gossip.Neighbor) {

}
