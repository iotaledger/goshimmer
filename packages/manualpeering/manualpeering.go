package manualpeering

import (
	"context"
	"sync"
	"sync/atomic"
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

type ConnectionDirection int8

const (
	ConnDirectionOutbound ConnectionDirection = iota
	ConnDirectionInbound
)

//go:generate stringer -type=ConnectionDirection

type ConnectionStatus int8

const (
	ConnStatusDisconnected ConnectionStatus = iota
	ConnStatusConnected
)

//go:generate stringer -type=ConnectionStatus

type KnownPeer struct {
	peer          *peer.Peer
	connDirection ConnectionDirection
	connStatus    *atomic.Value
	removeCh      chan struct{}
	doneCh        chan struct{}
}

func newKnownPeer(p *peer.Peer, connDirection ConnectionDirection) *KnownPeer {
	kp := &KnownPeer{
		peer:          p,
		connDirection: connDirection,
		connStatus:    &atomic.Value{},
		removeCh:      make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	kp.setConnStatus(ConnStatusDisconnected)
	return kp
}

func (kp *KnownPeer) ConnStatus() ConnectionStatus {
	return kp.connStatus.Load().(ConnectionStatus)
}

func (kp *KnownPeer) setConnStatus(cs ConnectionStatus) {
	kp.connStatus.Store(cs)
}

func (kp *KnownPeer) Peer() *peer.Peer {
	return kp.peer
}

func (kp *KnownPeer) ConnDirection() ConnectionDirection {
	return kp.connDirection
}

// Manager is the core entity in the manualpeering package.
// It holds a list of known peers and constantly provisions it to the gossip layer.
// Its job is to keep in sync the list of known peers
// and the list of current manual neighbors connected in the gossip layer.
// If a new peer is added to known peers, manager will forward it to gossip and make sure it establishes a connection.
// And vice versa, if a peer is being removed from the list of known peers,
// manager will make sure gossip drops that connection.
// Manager also subscribes to the gossip events and in case the connection with a manual peer fails it will reconnect.
type Manager struct {
	gm                *gossip.Manager
	log               *logger.Logger
	local             *peer.Local
	startOnce         sync.Once
	isStarted         typeutils.AtomicBool
	stopOnce          sync.Once
	stopMutex         sync.RWMutex
	isStopped         bool
	reconnectInterval time.Duration
	knownPeersMutex   sync.RWMutex
	knownPeers        map[identity.ID]*KnownPeer

	onGossipNeighborRemovedClosure *events.Closure
	onGossipNeighborAddedClosure   *events.Closure
}

// NewManager initializes a new Manager instance.
func NewManager(gm *gossip.Manager, local *peer.Local, log *logger.Logger) *Manager {
	m := &Manager{
		gm:                gm,
		local:             local,
		log:               log,
		reconnectInterval: defaultReconnectInterval,
		knownPeers:        map[identity.ID]*KnownPeer{},
	}
	m.onGossipNeighborRemovedClosure = events.NewClosure(m.onGossipNeighborRemoved)
	m.onGossipNeighborAddedClosure = events.NewClosure(m.onGossipNeighborAdded)
	return m
}

// AddPeer adds multiple peers to the list of known peers.
func (m *Manager) AddPeer(peers ...*peer.Peer) error {
	var resultErr error
	for _, p := range peers {
		if err := m.addPeer(p); err != nil {
			resultErr = errors.CombineErrors(resultErr, err)
		}
	}
	return resultErr
}

// RemovePeer removes multiple peers from the list of known peers.
func (m *Manager) RemovePeer(keys ...ed25519.PublicKey) error {
	var resultErr error
	for _, key := range keys {
		if err := m.removePeer(key); err != nil {
			resultErr = errors.CombineErrors(resultErr, err)
		}
	}
	return resultErr
}

type getKnownPeersConfig struct {
	onlyConnected bool
}

type GetKnownPeersOption func(options *getKnownPeersConfig)

func WithOnlyConnectedPeers() GetKnownPeersOption {
	return func(options *getKnownPeersConfig) {
		options.onlyConnected = true
	}
}

// GetKnownPeers returns the list of known peers.
func (m *Manager) GetKnownPeers(opts ...GetKnownPeersOption) []*KnownPeer {
	conf := &getKnownPeersConfig{}
	for _, o := range opts {
		o(conf)
	}
	m.knownPeersMutex.RLock()
	defer m.knownPeersMutex.RUnlock()
	peers := make([]*KnownPeer, 0, len(m.knownPeers))
	for _, p := range m.knownPeers {
		if !conf.onlyConnected || p.ConnStatus() == ConnStatusConnected {
			peers = append(peers, p)
		}
	}
	return peers
}

// Start subscribes to the gossip layer events and starts internal background workers.
// Calling multiple times has no effect.
func (m *Manager) Start() {
	m.startOnce.Do(func() {
		m.gm.NeighborsEvents(gossip.NeighborsGroupManual).NeighborRemoved.Attach(m.onGossipNeighborRemovedClosure)
		m.gm.NeighborsEvents(gossip.NeighborsGroupManual).NeighborAdded.Attach(m.onGossipNeighborAddedClosure)
		m.isStarted.Set()
	})
}

// Stop terminates internal background workers. Calling multiple times has no effect.
func (m *Manager) Stop() (err error) {
	if !m.isStarted.IsSet() {
		return errors.New("can't stop the manager: it hasn't been started yet")
	}
	m.stopOnce.Do(func() {
		m.stopMutex.Lock()
		defer m.stopMutex.Unlock()
		m.isStopped = true
		err = errors.WithStack(m.removeAllKnownPeers())
		m.gm.NeighborsEvents(gossip.NeighborsGroupManual).NeighborRemoved.Detach(m.onGossipNeighborRemovedClosure)
		m.gm.NeighborsEvents(gossip.NeighborsGroupManual).NeighborAdded.Detach(m.onGossipNeighborAddedClosure)
	})
	return err
}

func (m *Manager) addPeer(p *peer.Peer) error {
	if !m.isStarted.IsSet() {
		return errors.New("manualpeering manager hasn't been started yet")
	}
	m.stopMutex.RLock()
	defer m.stopMutex.RUnlock()
	if m.isStopped {
		return errors.New("manualpeering manager was stopped")
	}
	m.knownPeersMutex.Lock()
	defer m.knownPeersMutex.Unlock()
	connDirection, err := m.connectionDirection(p)
	if err != nil {
		return errors.WithStack(err)
	}
	kp := newKnownPeer(p, connDirection)
	if _, exists := m.knownPeers[kp.peer.ID()]; exists {
		return nil
	}
	m.knownPeers[kp.peer.ID()] = kp
	go func() {
		defer close(kp.doneCh)
		m.keepPeerConnected(kp)
	}()
	return nil
}

func (m *Manager) removePeer(key ed25519.PublicKey) error {
	m.knownPeersMutex.Lock()
	defer m.knownPeersMutex.Unlock()
	peerID := identity.NewID(key)
	err := m.removePeerByID(peerID)
	return errors.WithStack(err)
}

func (m *Manager) removeAllKnownPeers() error {
	m.knownPeersMutex.Lock()
	defer m.knownPeersMutex.Unlock()
	var resultErr error
	for peerID := range m.knownPeers {
		if err := m.removePeerByID(peerID); err != nil {
			resultErr = errors.CombineErrors(resultErr, err)
		}
	}
	return resultErr
}

func (m *Manager) removePeerByID(peerID identity.ID) error {
	kp, exists := m.knownPeers[peerID]
	if !exists {
		return nil
	}
	delete(m.knownPeers, peerID)
	<-kp.doneCh
	if err := m.gm.DropNeighbor(peerID, gossip.NeighborsGroupManual);
		err != nil && !errors.Is(err, gossip.ErrUnknownNeighbor) {
		return errors.Wrapf(err, "failed to drop known peer %s in the gossip layer", peerID)
	}
	return nil
}

func (m *Manager) keepPeerConnected(kp *KnownPeer) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	cancelContextOnRemove := func() {
		<-kp.removeCh
		ctxCancel()
	}
	go cancelContextOnRemove()

	ticker := time.NewTicker(m.reconnectInterval)
	defer ticker.Stop()

	peerID := kp.peer.ID()
	connectorFn := m.getConnectorFn(kp)
	for {
		if kp.ConnStatus() == ConnStatusDisconnected {
			if err := connectorFn(ctx, kp.peer, gossip.NeighborsGroupManual);
				err != nil && !errors.Is(err, gossip.ErrDuplicateNeighbor) && !errors.Is(err, context.Canceled) {
				m.log.Errorw(
					"Failed to connect a neighbor in the gossip layer",
					"peerID", peerID, "ConnectionDirection", kp.connDirection, "err", err,
				)
			}
		}
		select {
		case <-ticker.C:
		case <-kp.removeCh:
			<-ctx.Done()
			return
		}
	}
}

type connectorFunc func(context.Context, *peer.Peer, gossip.NeighborsGroup) error

func (m *Manager) getConnectorFn(kp *KnownPeer) connectorFunc {
	var fn connectorFunc
	if kp.connDirection == ConnDirectionOutbound {
		fn = m.gm.AddOutbound
	} else if kp.connDirection == ConnDirectionInbound {
		fn = m.gm.AddInbound
	}
	return fn
}

func (m *Manager) onGossipNeighborRemoved(neighbor *gossip.Neighbor) {
	m.changeNeighborStatus(neighbor, ConnStatusDisconnected)
}

func (m *Manager) onGossipNeighborAdded(neighbor *gossip.Neighbor) {
	m.changeNeighborStatus(neighbor, ConnStatusConnected)
}
func (m *Manager) changeNeighborStatus(neighbor *gossip.Neighbor, connStatus ConnectionStatus) {
	m.knownPeersMutex.RLock()
	defer m.knownPeersMutex.RUnlock()
	kp, exists := m.knownPeers[neighbor.ID()]
	if !exists {
		return
	}
	kp.setConnStatus(connStatus)
}

func (m *Manager) connectionDirection(p *peer.Peer) (ConnectionDirection, error) {
	localPK := m.local.PublicKey().String()
	peerPK := p.PublicKey().String()
	if localPK < peerPK {
		return ConnDirectionOutbound, nil
	} else if localPK > peerPK {
		return ConnDirectionInbound, nil
	} else {
		return ConnectionDirection(0), errors.Newf(
			"manualpeering: provided neighbor public key %s is the same as the local %s: can't compare lexicographically",
			peerPK,
			localPK,
		)
	}
}
