package gossip

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"

	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	// maxPacketSize defines the maximum packet size allowed for gossip and bufferedconn.
	maxPacketSize = 65 * 1024
)

var (
	messageWorkerCount     = runtime.GOMAXPROCS(0) * 4
	messageWorkerQueueSize = 1000

	messageRequestWorkerCount     = runtime.GOMAXPROCS(0)
	messageRequestWorkerQueueSize = 100
)

// LoadMessageFunc defines a function that returns the message for the given id.
type LoadMessageFunc func(messageId tangle.MessageID) ([]byte, error)

// ConnectPeerOption defines an option for the DialPeer and AcceptPeer methods.
type ConnectPeerOption func(conf *connectPeerConfig)

type connectPeerConfig struct {
	useDefaultTimeout bool
}

func buildConnectPeerConfig(opts []ConnectPeerOption) *connectPeerConfig {
	conf := &connectPeerConfig{
		useDefaultTimeout: true,
	}
	for _, o := range opts {
		o(conf)
	}
	return conf
}

// WithNoDefaultTimeout returns a ConnectPeerOption that disables the default timeout for dial or accept.
func WithNoDefaultTimeout() ConnectPeerOption {
	return func(conf *connectPeerConfig) {
		conf.useDefaultTimeout = false
	}
}

// The Manager handles the connected neighbors.
type Manager struct {
	local           *peer.Local
	host            host.Host
	networkNotifiee *networkNotifiee
	closing   chan struct{} // if this channel gets closed all pending waits should terminate

	acceptWG    sync.WaitGroup
	acceptMutex sync.RWMutex
	acceptMap   map[libp2ppeer.ID]*acceptMatcher

	loadMessageFunc LoadMessageFunc
	log             *logger.Logger
	events          Events
	neighborsEvents map[NeighborsGroup]NeighborsEvents

	server      *server.TCP
	serverMutex sync.RWMutex

	neighbors      map[identity.ID]*Neighbor
	neighborsMutex sync.RWMutex

	// messageWorkerPool defines a worker pool where all incoming messages are processed.
	messageWorkerPool *workerpool.NonBlockingQueuedWorkerPool

	messageRequestWorkerPool *workerpool.NonBlockingQueuedWorkerPool
}

// NewManager creates a new Manager.
func NewManager(host host.Host, local *peer.Local, f LoadMessageFunc, log *logger.Logger) *Manager {
	m := &Manager{
		host:            host,
		closing:          make(chan struct{}),
		acceptMap:       map[libp2ppeer.ID]*acceptMatcher{},
		local:           local,
		loadMessageFunc: f,
		log:             log,
		events: Events{
			MessageReceived: events.NewEvent(messageReceived),
		},
		neighborsEvents: map[NeighborsGroup]NeighborsEvents{
			NeighborsGroupAuto:   NewNeighborsEvents(),
			NeighborsGroupManual: NewNeighborsEvents(),
		},
		neighbors: map[identity.ID]*Neighbor{},
		server:    nil,
	}
	m.messageWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		m.processPacketMessage(task.Param(0).([]byte), task.Param(1).(*Neighbor))

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))

	m.messageRequestWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		m.processMessageRequest(task.Param(0).([]byte), task.Param(1).(*Neighbor))

		task.Return(nil)
	}, workerpool.WorkerCount(messageRequestWorkerCount), workerpool.QueueSize(messageRequestWorkerQueueSize))
	nn := &networkNotifiee{m: m}
	m.networkNotifiee = nn

	m.host.SetStreamHandler(protocolID, m.streamHandler)
	m.host.Network().Notify(nn)

	return m
}

type networkNotifiee struct {
	m *Manager
}

func (nn *networkNotifiee) Listen(_ network.Network, _ multiaddr.Multiaddr)      {}
func (nn *networkNotifiee) ListenClose(_ network.Network, _ multiaddr.Multiaddr) {}
func (nn *networkNotifiee) Connected(_ network.Network, _ network.Conn)          {}
func (nn *networkNotifiee) Disconnected(_ network.Network, _ network.Conn)       {}
func (nn *networkNotifiee) OpenedStream(_ network.Network, _ network.Stream)     {}
func (nn *networkNotifiee) ClosedStream(n network.Network, stream network.Stream) {

}

// Start starts the manager for the given TCP server.
func (m *Manager) Start(srv *server.TCP) {
	m.serverMutex.Lock()
	defer m.serverMutex.Unlock()

	m.server = srv
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.serverMutex.Lock()
	defer m.serverMutex.Unlock()

	m.server = nil

	m.dropAllNeighbors()

	m.messageWorkerPool.Stop()
	m.messageRequestWorkerPool.Stop()
}

func (m *Manager) dropAllNeighbors() {
	neighborsList := m.AllNeighbors()
	for _, nbr := range neighborsList {
		_ = nbr.Close()
	}
}

// Events returns the events related to the gossip protocol.
func (m *Manager) Events() Events {
	return m.events
}

// NeighborsEvents returns the events related to the gossip protocol.
func (m *Manager) NeighborsEvents(group NeighborsGroup) NeighborsEvents {
	return m.neighborsEvents[group]
}

// AddOutbound tries to add a neighbor by connecting to that peer.
func (m *Manager) AddOutbound(ctx context.Context, p *peer.Peer, group NeighborsGroup,
	connectOpts ...server.ConnectPeerOption) error {
	return m.addNeighbor(ctx, p, group, m.server.DialPeer, connectOpts)
}

// AddInbound tries to add a neighbor by accepting an incoming connection from that peer.
func (m *Manager) AddInbound(ctx context.Context, p *peer.Peer, group NeighborsGroup,
	connectOpts ...server.ConnectPeerOption) error {
	p.PublicKey().Bytes()
	return m.addNeighbor(ctx, p, group, m.server.AcceptPeer, connectOpts)
}

// DropNeighbor disconnects the neighbor with the given ID and the group.
func (m *Manager) DropNeighbor(id identity.ID, group NeighborsGroup) error {
	nbr, err := m.getNeighbor(id, group)
	if err != nil {
		return errors.WithStack(err)
	}
	return nbr.Close()
}

// getNeighbor returns neighbor by ID and group.
func (m *Manager) getNeighbor(id identity.ID, group NeighborsGroup) (*Neighbor, error) {
	m.neighborsMutex.RLock()
	defer m.neighborsMutex.RUnlock()
	nbr, ok := m.neighbors[id]
	if !ok || nbr.Group != group {
		return nil, ErrUnknownNeighbor
	}
	return nbr, nil
}

// RequestMessage requests the message with the given id from the neighbors.
// If no peer is provided, all neighbors are queried.
func (m *Manager) RequestMessage(messageID []byte, to ...identity.ID) {
	msgReq := &pb.MessageRequest{Id: messageID}
	m.send(marshal(msgReq), to...)
}

// SendMessage adds the given message the send queue of the neighbors.
// The actual send then happens asynchronously. If no peer is provided, it is send to all neighbors.
func (m *Manager) SendMessage(msgData []byte, to ...identity.ID) {
	msg := &pb.Message{Data: msgData}
	m.send(marshal(msg), to...)
}

// AllNeighbors returns all the neighbors that are currently connected.
func (m *Manager) AllNeighbors() []*Neighbor {
	m.neighborsMutex.RLock()
	defer m.neighborsMutex.RUnlock()
	result := make([]*Neighbor, 0, len(m.neighbors))
	for _, n := range m.neighbors {
		result = append(result, n)
	}
	return result
}

func (m *Manager) getNeighborsByID(ids []identity.ID) []*Neighbor {
	result := make([]*Neighbor, 0, len(ids))
	if len(ids) == 0 {
		return result
	}

	m.neighborsMutex.RLock()
	defer m.neighborsMutex.RUnlock()
	for _, id := range ids {
		if n, ok := m.neighbors[id]; ok {
			result = append(result, n)
		}
	}
	return result
}

func (m *Manager) send(b []byte, to ...identity.ID) {
	neighbors := m.getNeighborsByID(to)
	if len(neighbors) == 0 {
		neighbors = m.AllNeighbors()
	}

	for _, nbr := range neighbors {
		if _, err := nbr.Write(b); err != nil {
			m.log.Warnw("send error", "peer-id", nbr.ID(), "err", err)
		}
	}
}

func (m *Manager) addNeighbor(ctx context.Context, p *peer.Peer, group NeighborsGroup,
	connectorFunc func(context.Context, *peer.Peer, ...server.ConnectPeerOption) (net.Conn, error),
	connectOpts []server.ConnectPeerOption,
) error {
	if p.ID() == m.local.ID() {
		return ErrLoopbackNeighbor
	}
	m.serverMutex.RLock()
	defer m.serverMutex.RUnlock()
	if m.server == nil {
		return ErrNotRunning
	}
	if m.neighborExists(p.ID()) {
		m.neighborsEvents[group].ConnectionFailed.Trigger(p, ErrDuplicateNeighbor)
		return ErrDuplicateNeighbor
	}

	conn, err := connectorFunc(ctx, p, connectOpts...)
	if err != nil {
		m.neighborsEvents[group].ConnectionFailed.Trigger(p, err)
		return err
	}

	// create and add the neighbor
	nbr := NewNeighbor(p, group, conn, m.log)
	if err := m.setNeighbor(nbr); err != nil {
		_ = conn.Close()
		m.neighborsEvents[group].ConnectionFailed.Trigger(p, err)
		return errors.WithStack(err)
	}
	m.neighborsEvents[group].NeighborAdded.Trigger(nbr)

	return nil
}

func (m *Manager) neighborExists(id identity.ID) bool {
	m.neighborsMutex.RLock()
	defer m.neighborsMutex.RUnlock()
	_, ok := m.neighbors[id]
	return ok
}

func (m *Manager) deleteNeighbor(nbr *Neighbor) {
	m.neighborsMutex.Lock()
	defer m.neighborsMutex.Unlock()
	delete(m.neighbors, nbr.ID())
}

func (m *Manager) setNeighbor(nbr *Neighbor) error {
	m.neighborsMutex.Lock()
	defer m.neighborsMutex.Unlock()
	if _, exists := m.neighbors[nbr.ID()]; exists {
		return errors.WithStack(ErrDuplicateNeighbor)
	}
	nbr.Events.Close.Attach(events.NewClosure(func() {
		// assure that the neighbor is removed and notify
		m.deleteNeighbor(nbr)
		m.neighborsEvents[nbr.Group].NeighborRemoved.Trigger(nbr)
	}))
	nbr.Events.ReceiveMessage.Attach(events.NewClosure(func(data []byte) {
		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)
		if err := m.handlePacket(dataCopy, nbr); err != nil {
			m.log.Debugw("error handling packet", "err", err)
		}
	}))
	m.neighbors[nbr.ID()] = nbr
	nbr.Listen()
	return nil
}

func (m *Manager) handlePacket(data []byte, nbr *Neighbor) error {
	// ignore empty packages
	if len(data) == 0 {
		return nil
	}

	switch pb.PacketType(data[0]) {
	case pb.PacketMessage:
		if _, added := m.messageWorkerPool.TrySubmit(data, nbr); !added {
			return fmt.Errorf("messageWorkerPool full: packet message discarded")
		}
	case pb.PacketMessageRequest:
		if _, added := m.messageRequestWorkerPool.TrySubmit(data, nbr); !added {
			return fmt.Errorf("messageRequestWorkerPool full: message request discarded")
		}

	default:
		return ErrInvalidPacket
	}

	return nil
}

func marshal(packet pb.Packet) []byte {
	packetType := packet.Type()
	if packetType > 0xFF {
		panic("invalid packet")
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		panic("invalid packet")
	}
	return append([]byte{byte(packetType)}, data...)
}

// MessageWorkerPoolStatus returns the name and the load of the workerpool.
func (m *Manager) MessageWorkerPoolStatus() (name string, load int) {
	return "messageWorkerPool", m.messageWorkerPool.GetPendingQueueSize()
}

// MessageRequestWorkerPoolStatus returns the name and the load of the workerpool.
func (m *Manager) MessageRequestWorkerPoolStatus() (name string, load int) {
	return "messageRequestWorkerPool", m.messageRequestWorkerPool.GetPendingQueueSize()
}

func (m *Manager) processPacketMessage(data []byte, nbr *Neighbor) {
	packet := new(pb.Message)
	if err := proto.Unmarshal(data[1:], packet); err != nil {
		m.log.Debugw("error processing packet", "err", err)
		return
	}
	m.events.MessageReceived.Trigger(&MessageReceivedEvent{Data: packet.GetData(), Peer: nbr.Peer})
}

func (m *Manager) processMessageRequest(data []byte, nbr *Neighbor) {
	packet := new(pb.MessageRequest)
	if err := proto.Unmarshal(data[1:], packet); err != nil {
		m.log.Debugw("invalid packet", "err", err)
		return
	}

	msgID, _, err := tangle.MessageIDFromBytes(packet.GetId())
	if err != nil {
		m.log.Debugw("invalid message id:", "err", err)
		return
	}

	msgBytes, err := m.loadMessageFunc(msgID)
	if err != nil {
		m.log.Debugw("error loading message", "msg-id", msgID, "err", err)
		return
	}

	// send the loaded message directly to the neighbor
	_, _ = nbr.Write(marshal(&pb.Message{Data: msgBytes}))
}

func toLibp2pID(p *peer.Peer) (libp2ppeer.ID, error) {
	pubKeyBytes := p.PublicKey().Bytes()
	pubKey, err := crypto.UnmarshalEd25519PublicKey(pubKeyBytes)
	if err != nil {
		return "", errors.WithStack(err)
	}
	libp2pID, err := libp2ppeer.IDFromPublicKey(pubKey)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return libp2pID, nil
}
