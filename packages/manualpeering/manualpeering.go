package manualpeering

import (
	"bytes"
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/typeutils"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/gossip"
)

const defaultReconnectInterval = 5 * time.Second

// ConnectionDirection is an enum for the type of connection between local peer and the other peer in the gossip layer.
type ConnectionDirection string

const (
	// ConnDirectionOutbound means that the local peer dials for the connection in the gossip layer.
	ConnDirectionOutbound ConnectionDirection = "outbound"
	// ConnDirectionInbound means that the local peer accepts for the connection in the gossip layer.
	ConnDirectionInbound ConnectionDirection = "inbound"
)

// ConnectionStatus is an enum for the peer connection status in the gossip layer.
type ConnectionStatus string

const (
	// ConnStatusDisconnected means that there is no real connection established in the gossip layer for that peer.
	ConnStatusDisconnected ConnectionStatus = "disconnected"
	// ConnStatusConnected means that there is a real connection established in the gossip layer for that peer.
	ConnStatusConnected ConnectionStatus = "connected"
)

// KnownPeerToAdd defines a type that is used in .AddPeer() method.
type KnownPeerToAdd struct {
	PublicKey ed25519.PublicKey `json:"publicKey"`
	Address   string            `json:"address"`
}

// KnownPeer defines a peer record in the manual peering layer.
type KnownPeer struct {
	PublicKey     ed25519.PublicKey   `json:"publicKey"`
	Address       string              `json:"address"`
	ConnDirection ConnectionDirection `json:"connectionDirection"`
	ConnStatus    ConnectionStatus    `json:"connectionStatus"`
}

// Manager is the core entity in the manual peering package.
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
	knownPeers        map[identity.ID]*knownPeer

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
		knownPeers:        map[identity.ID]*knownPeer{},
	}
	m.onGossipNeighborRemovedClosure = events.NewClosure(m.onGossipNeighborRemoved)
	m.onGossipNeighborAddedClosure = events.NewClosure(m.onGossipNeighborAdded)
	return m
}

// AddPeer adds multiple peers to the list of known peers.
func (m *Manager) AddPeer(peers ...*KnownPeerToAdd) error {
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

// GetPeersConfig holds optional parameters for the GetPeers method.
type GetPeersConfig struct {
	// If true, GetPeers returns peers that have actual connection established in the gossip layer.
	OnlyConnected bool `json:"onlyConnected"`
}

// GetPeersOption defines a single option for GetPeers method.
type GetPeersOption func(conf *GetPeersConfig)

// BuildGetPeersConfig builds GetPeersConfig struct from a list of options.
func BuildGetPeersConfig(opts []GetPeersOption) *GetPeersConfig {
	conf := &GetPeersConfig{}
	for _, o := range opts {
		o(conf)
	}
	return conf
}

// ToOptions translates config struct to a list of corresponding options.
func (c *GetPeersConfig) ToOptions() (opts []GetPeersOption) {
	if c.OnlyConnected {
		opts = append(opts, WithOnlyConnectedPeers())
	}
	return opts
}

// WithOnlyConnectedPeers returns a GetPeersOption that sets OnlyConnected field to true.
func WithOnlyConnectedPeers() GetPeersOption {
	return func(conf *GetPeersConfig) {
		conf.OnlyConnected = true
	}
}

// GetPeers returns the list of known peers.
func (m *Manager) GetPeers(opts ...GetPeersOption) []*KnownPeer {
	conf := BuildGetPeersConfig(opts)
	m.knownPeersMutex.RLock()
	defer m.knownPeersMutex.RUnlock()
	peers := make([]*KnownPeer, 0, len(m.knownPeers))
	for _, kp := range m.knownPeers {
		connStatus := kp.getConnStatus()
		if !conf.OnlyConnected || connStatus == ConnStatusConnected {
			peers = append(peers, &KnownPeer{
				PublicKey:     kp.peer.PublicKey(),
				Address:       kp.peerAddress,
				ConnDirection: kp.connDirection,
				ConnStatus:    connStatus,
			})
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

type knownPeer struct {
	peer          *peer.Peer
	peerAddress   string
	connDirection ConnectionDirection
	connStatus    *atomic.Value
	removeCh      chan struct{}
	doneCh        chan struct{}
}

func newKnownPeer(p *KnownPeerToAdd, connDirection ConnectionDirection) (*knownPeer, error) {
	tcpAddress, err := net.ResolveTCPAddr("tcp", p.Address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse peer address")
	}
	services := service.New()
	// Peering key is required in order to initialize a peer,
	// but it's not used in both manual peering and gossip layers so we just specify the default one.
	services.Update(service.PeeringKey, "tcp", 14626)
	services.Update(service.GossipKey, tcpAddress.Network(), tcpAddress.Port)
	kp := &knownPeer{
		peer:          peer.NewPeer(identity.New(p.PublicKey), tcpAddress.IP, services),
		peerAddress:   p.Address,
		connDirection: connDirection,
		connStatus:    &atomic.Value{},
		removeCh:      make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	kp.setConnStatus(ConnStatusDisconnected)
	return kp, nil
}

func (kp *knownPeer) getConnStatus() ConnectionStatus {
	return kp.connStatus.Load().(ConnectionStatus)
}

func (kp *knownPeer) setConnStatus(cs ConnectionStatus) {
	kp.connStatus.Store(cs)
}

func (m *Manager) addPeer(p *KnownPeerToAdd) error {
	if !m.isStarted.IsSet() {
		return errors.New("manual peering manager hasn't been started yet")
	}
	m.stopMutex.RLock()
	defer m.stopMutex.RUnlock()
	if m.isStopped {
		return errors.New("manual peering manager was stopped")
	}
	m.knownPeersMutex.Lock()
	defer m.knownPeersMutex.Unlock()
	connDirection, err := m.connectionDirection(p.PublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	kp, err := newKnownPeer(p, connDirection)
	if err != nil {
		return errors.WithStack(err)
	}
	if _, exists := m.knownPeers[kp.peer.ID()]; exists {
		return nil
	}
	m.log.Infow("Adding new peer to the list of known peers in manual peering", "peer", p)
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
	m.log.Infow("Removing peer from from the list of known peers in manual peering",
		"publicKey", key)
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
	close(kp.removeCh)
	<-kp.doneCh
	if err := m.gm.DropNeighbor(peerID, gossip.NeighborsGroupManual); err != nil && !errors.Is(err, gossip.ErrUnknownNeighbor) {
		return errors.Wrapf(err, "failed to drop known peer %s in the gossip layer", peerID)
	}
	return nil
}

func (m *Manager) keepPeerConnected(kp *knownPeer) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	cancelContextOnRemove := func() {
		<-kp.removeCh
		ctxCancel()
	}
	go cancelContextOnRemove()

	ticker := time.NewTicker(m.reconnectInterval)
	defer ticker.Stop()

	peerID := kp.peer.ID()
	for {
		if kp.getConnStatus() == ConnStatusDisconnected {
			m.log.Infow(
				"Peer is disconnected, calling gossip layer to establish the connection",
				"peer", kp.peer, "connectionDirection", kp.connDirection,
			)
			var err error
			if kp.connDirection == ConnDirectionOutbound {
				err = m.gm.AddOutbound(ctx, kp.peer, gossip.NeighborsGroupManual)
			} else if kp.connDirection == ConnDirectionInbound {
				err = m.gm.AddInbound(ctx, kp.peer, gossip.NeighborsGroupManual, gossip.WithNoDefaultTimeout())
			}
			if err != nil && !errors.Is(err, gossip.ErrDuplicateNeighbor) && !errors.Is(err, context.Canceled) {
				m.log.Errorw(
					"Failed to connect a neighbor in the gossip layer",
					"peerID", peerID, "connectionDirection", kp.connDirection, "err", err,
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

func (m *Manager) onGossipNeighborRemoved(neighbor *gossip.Neighbor) {
	m.changeNeighborStatus(neighbor, ConnStatusDisconnected)
}

func (m *Manager) onGossipNeighborAdded(neighbor *gossip.Neighbor) {
	m.changeNeighborStatus(neighbor, ConnStatusConnected)
	m.log.Infow(
		"Gossip layer successfully connected with the peer",
		"peer", neighbor.Peer,
	)
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

func (m *Manager) connectionDirection(peerPK ed25519.PublicKey) (ConnectionDirection, error) {
	result := bytes.Compare(m.local.PublicKey().Bytes(), peerPK.Bytes())
	if result < 0 {
		return ConnDirectionOutbound, nil
	} else if result > 0 {
		return ConnDirectionInbound, nil
	} else {
		return "", errors.Newf(
			"manual peering: provided neighbor public key %s is the same as the local %s: can't compare lexicographically",
			peerPK,
			m.local.PublicKey(),
		)
	}
}
