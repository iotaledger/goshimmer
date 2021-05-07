package manualpeering

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/typeutils"

	"github.com/iotaledger/goshimmer/packages/gossip"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
)

const defaultReconnectInterval = 5 * time.Second

// Manager is the core entity in the manualpeering package.
// It holds a list of known peers and constantly provisions it to the gossip layer.
// Its job is to keep in sync the list of known peers
// and the list of current manual neighbors connected in the gossip layer.
// If a new peer is added to known peers, manager will forward it to gossip and make sure it establishes a connection.
// And vice versa, if a peer is being removed from the list of known peers,
// manager will make sure gossip drops that connection.
// Manager also subscribes to the gossip events and in case the connection with a manual peer fails it will reconnect.
type Manager struct {
	gm                      *gossip.Manager
	log                     *logger.Logger
	local                   *peer.Local
	startOnce               sync.Once
	isStarted               typeutils.AtomicBool
	stopOnce                sync.Once
	stopCh                  chan struct{}
	workersWG               sync.WaitGroup
	syncTicker              *time.Ticker
	syncTriggerCh           chan struct{}
	knownPeersMutex         sync.RWMutex
	knownPeers              map[identity.ID]*peer.Peer
	connectedNeighborsMutex sync.RWMutex
	connectedNeighbors      map[identity.ID]*peer.Peer

	onGossipNeighborRemovedClosure *events.Closure
}

// NewManager initializes a new Manager instance.
func NewManager(gm *gossip.Manager, local *peer.Local, log *logger.Logger) *Manager {
	m := &Manager{
		gm:                 gm,
		local:              local,
		log:                log,
		syncTicker:         time.NewTicker(defaultReconnectInterval),
		syncTriggerCh:      make(chan struct{}, 1),
		stopCh:             make(chan struct{}),
		knownPeers:         map[identity.ID]*peer.Peer{},
		connectedNeighbors: map[identity.ID]*peer.Peer{},
	}
	m.onGossipNeighborRemovedClosure = events.NewClosure(m.onGossipNeighborRemoved)
	return m
}

// AddPeer adds multiple peers to the list of known peers.
func (m *Manager) AddPeer(peers ...*peer.Peer) {
	m.mutateKnownPeers(func(knownPeers map[identity.ID]*peer.Peer) {
		for _, p := range peers {
			knownPeers[p.ID()] = p
		}
	})
}

// RemovePeer removes multiple peers from the list of known peers.
func (m *Manager) RemovePeer(keys ...ed25519.PublicKey) {
	ids := make([]identity.ID, len(keys))
	for i, key := range keys {
		ids[i] = identity.NewID(key)
	}
	m.mutateKnownPeers(func(knownPeers map[identity.ID]*peer.Peer) {
		for _, id := range ids {
			delete(knownPeers, id)
		}
	})
}

// GetKnownPeers returns the list of known peers.
func (m *Manager) GetKnownPeers() []*peer.Peer {
	m.knownPeersMutex.RLock()
	defer m.knownPeersMutex.RUnlock()
	peers := make([]*peer.Peer, 0, len(m.knownPeers))
	for _, p := range m.knownPeers {
		peers = append(peers, p)
	}
	return peers
}

// GetConnectedPeers returns the list of connected peers.
func (m *Manager) GetConnectedPeers() []*peer.Peer {
	m.connectedNeighborsMutex.RLock()
	defer m.connectedNeighborsMutex.RUnlock()
	peers := make([]*peer.Peer, 0, len(m.connectedNeighbors))
	for _, p := range m.connectedNeighbors {
		peers = append(peers, p)
	}
	return peers
}

func (m *Manager) mutateKnownPeers(mutateFn func(knownPeers map[identity.ID]*peer.Peer)) {
	m.knownPeersMutex.Lock()
	defer m.knownPeersMutex.Unlock()

	mutateFn(m.knownPeers)

	m.triggerSync()
}

// Start subscribes to the gossip layer events and starts internal background workers.
// Calling multiple times has no effect.
func (m *Manager) Start() {
	m.startOnce.Do(func() {
		m.gm.NeighborsEvents(gossip.NeighborsGroupManual).NeighborRemoved.Attach(m.onGossipNeighborRemovedClosure)
		m.workersWG.Add(1)
		go func() {
			defer m.workersWG.Done()
			m.syncGossipNeighbors()
		}()
		m.isStarted.Set()
	})
}

// Stop terminates internal background workers. Calling multiple times has no effect.
func (m *Manager) Stop() error {
	if !m.isStarted.IsSet() {
		return errors.New("can't stop the manager: it hasn't been started yet")
	}
	m.stopOnce.Do(func() {
		m.gm.NeighborsEvents(gossip.NeighborsGroupManual).NeighborRemoved.Detach(m.onGossipNeighborRemovedClosure)
		close(m.stopCh)
		m.workersWG.Wait()
	})
	return nil
}

func (m *Manager) onGossipNeighborRemoved(neighbor *gossip.Neighbor) {
	m.connectedNeighborsMutex.Lock()
	defer m.connectedNeighborsMutex.Unlock()
	delete(m.connectedNeighbors, neighbor.ID())
}

func (m *Manager) syncGossipNeighbors() {
	for {
		select {
		case <-m.stopCh:
			return
		case <-m.syncTriggerCh:
		case <-m.syncTicker.C:
		}

		neighborsToConnect, neighborsToDrop := m.getNeighborsDiff()
		wg := sync.WaitGroup{}
		const tasksNum = 2
		wg.Add(tasksNum)

		go func() {
			defer wg.Done()
			m.connectNeighbors(neighborsToConnect)
		}()
		go func() {
			defer wg.Done()
			m.dropNeighbors(neighborsToDrop)
		}()

		wg.Wait()
	}
}

func (m *Manager) getNeighborsDiff() (neighborsToConnect, neighborsToDrop []*peer.Peer) {
	m.connectedNeighborsMutex.RLock()
	defer m.connectedNeighborsMutex.RUnlock()
	m.knownPeersMutex.RLock()
	defer m.knownPeersMutex.RUnlock()

	for peerID, p := range m.knownPeers {
		if _, exists := m.connectedNeighbors[peerID]; !exists {
			neighborsToConnect = append(neighborsToConnect, p)
		}
	}

	for peerID, p := range m.connectedNeighbors {
		if _, exists := m.knownPeers[peerID]; !exists {
			neighborsToDrop = append(neighborsToDrop, p)
		}
	}

	return neighborsToConnect, neighborsToDrop
}

func (m *Manager) connectNeighbors(peers []*peer.Peer) {
	if len(peers) == 0 {
		return
	}
	m.log.Infow("Found known peers that aren't in the gossip neighbors list, connecting to them", "peers", peers)
	wg := sync.WaitGroup{}
	wg.Add(len(peers))
	for _, p := range peers {
		go func(p *peer.Peer) {
			defer wg.Done()
			if err := m.connectNeighbor(p); err != nil {
				m.log.Errorw("Failed to add a neighbor to gossip layer, skip it", "err", err)
			}
		}(p)
	}
	wg.Wait()
}

func (m *Manager) dropNeighbors(peers []*peer.Peer) {
	if len(peers) == 0 {
		return
	}
	m.log.Infow("Found gossip neighbors that aren't in the known peers anymore, dropping them", "peers", peers)
	wg := sync.WaitGroup{}
	wg.Add(len(peers))
	for _, p := range peers {
		go func(p *peer.Peer) {
			defer wg.Done()
			if err := m.dropNeighbor(p); err != nil {
				m.log.Errorw("Failed to drop a neighbor in gossip layer, skip it", "err", err)
			}
		}(p)
	}
	wg.Wait()
}

func (m *Manager) connectNeighbor(p *peer.Peer) error {
	connDirection, err := m.connectionDirection(p)
	if err != nil {
		return errors.Wrap(err, "failed to figure out the connection direction for peer")
	}
	switch connDirection {
	case connDirectionOutbound:
		if err := m.gm.AddOutbound(p, gossip.NeighborsGroupManual); err != nil {
			return errors.Wrapf(err, "failed to connect an outbound neighbor; publicKey=%s", p.PublicKey())
		}
	case connDirectionInbound:
		if err := m.gm.AddInbound(p, gossip.NeighborsGroupManual); err != nil {
			return errors.Wrapf(err, "failed to connect an inbound neighbor; publicKey=%s", p.PublicKey())
		}
	default:
		return errors.Newf("unknown connection direction for neighbor; publicKey=%s, connDirection=%s", p.PublicKey(), connDirection)
	}
	m.connectedNeighborsMutex.Lock()
	defer m.connectedNeighborsMutex.Unlock()
	m.connectedNeighbors[p.ID()] = p
	return nil
}

func (m *Manager) dropNeighbor(p *peer.Peer) error {
	if err := m.gm.DropNeighbor(p.ID(), gossip.NeighborsGroupManual); err != nil {
		return errors.Wrapf(err, "gossip layer failed to drop neighbor %s", p.PublicKey())
	}
	m.connectedNeighborsMutex.Lock()
	defer m.connectedNeighborsMutex.Unlock()
	delete(m.connectedNeighbors, p.ID())
	return nil
}

func (m *Manager) triggerSync() {
	select {
	case m.syncTriggerCh <- struct{}{}:
	default:
	}
}

type connectionDirection int8

const (
	connDirectionOutbound connectionDirection = iota
	connDirectionInbound
)

//go:generate stringer -type=connectionDirection

func (m *Manager) connectionDirection(p *peer.Peer) (connectionDirection, error) {
	localPK := m.local.PublicKey().String()
	peerPK := p.PublicKey().String()
	if localPK < peerPK {
		return connDirectionOutbound, nil
	} else if localPK > peerPK {
		return connDirectionInbound, nil
	} else {
		return connectionDirection(0), errors.Newf(
			"manualpeering: provided neighbor public key %s is the same as the local %s: can't compare lexicographically",
			peerPK,
			localPK,
		)
	}
}
