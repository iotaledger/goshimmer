package gossip

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/libp2p/go-libp2p-core/host"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"

	pb "github.com/iotaledger/goshimmer/packages/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/ratelimiter"
	"github.com/iotaledger/goshimmer/packages/tangle"
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
	local      *peer.Local
	Libp2pHost host.Host

	acceptWG    sync.WaitGroup
	acceptMutex sync.RWMutex
	acceptMap   map[libp2ppeer.ID]*acceptMatcher

	loadMessageFunc LoadMessageFunc
	log             *logger.Logger
	events          Events
	neighborsEvents map[NeighborsGroup]NeighborsEvents

	stopMutex sync.RWMutex
	isStopped bool

	neighbors      map[identity.ID]*Neighbor
	neighborsMutex sync.RWMutex

	messagesRateLimiter        *ratelimiter.PeerRateLimiter
	messageRequestsRateLimiter *ratelimiter.PeerRateLimiter

	// messageWorkerPool defines a worker pool where all incoming messages are processed.
	messageWorkerPool *workerpool.NonBlockingQueuedWorkerPool

	messageRequestWorkerPool *workerpool.NonBlockingQueuedWorkerPool
}

// ManagerOption configures the Manager instance.
type ManagerOption func(m *Manager)

// NewManager creates a new Manager.
func NewManager(libp2pHost host.Host, local *peer.Local, f LoadMessageFunc, log *logger.Logger, opts ...ManagerOption,
) *Manager {
	m := &Manager{
		Libp2pHost:      libp2pHost,
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
	}
	m.messageWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		m.processMessagePacket(task.Param(0).(*pb.Packet_Message), task.Param(1).(*Neighbor))

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))

	m.messageRequestWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		m.processMessageRequestPacket(task.Param(0).(*pb.Packet_MessageRequest), task.Param(1).(*Neighbor))

		task.Return(nil)
	}, workerpool.WorkerCount(messageRequestWorkerCount), workerpool.QueueSize(messageRequestWorkerQueueSize))

	m.Libp2pHost.SetStreamHandler(protocolID, m.streamHandler)
	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithMessagesRateLimiter allows to set a PeerRateLimiter instance
// to be used as messages rate limiter in the gossip manager.
func WithMessagesRateLimiter(prl *ratelimiter.PeerRateLimiter) ManagerOption {
	return func(m *Manager) {
		m.messagesRateLimiter = prl
	}
}

// MessagesRateLimiter returns the messages rate limiter instance used in the gossip manager.
func (m *Manager) MessagesRateLimiter() *ratelimiter.PeerRateLimiter {
	return m.messagesRateLimiter
}

// WithMessageRequestsRateLimiter allows to set a PeerRateLimiter instance
// to be used as messages requests rate limiter in the gossip manager.
func WithMessageRequestsRateLimiter(prl *ratelimiter.PeerRateLimiter) ManagerOption {
	return func(m *Manager) {
		m.messageRequestsRateLimiter = prl
	}
}

// MessageRequestsRateLimiter returns the message requests rate limiter instance used in the gossip manager.
func (m *Manager) MessageRequestsRateLimiter() *ratelimiter.PeerRateLimiter {
	return m.messageRequestsRateLimiter
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.stopMutex.Lock()
	defer m.stopMutex.Unlock()

	if m.isStopped {
		return
	}
	m.isStopped = true
	m.Libp2pHost.RemoveStreamHandler(protocolID)
	m.dropAllNeighbors()

	m.messageWorkerPool.Stop()
	m.messageRequestWorkerPool.Stop()
}

func (m *Manager) dropAllNeighbors() {
	neighborsList := m.AllNeighbors()
	for _, nbr := range neighborsList {
		nbr.close()
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
	connectOpts ...ConnectPeerOption) error {
	return m.addNeighbor(ctx, p, group, m.dialPeer, connectOpts)
}

// AddInbound tries to add a neighbor by accepting an incoming connection from that peer.
func (m *Manager) AddInbound(ctx context.Context, p *peer.Peer, group NeighborsGroup,
	connectOpts ...ConnectPeerOption) error {
	return m.addNeighbor(ctx, p, group, m.acceptPeer, connectOpts)
}

// GetNeighbor returns the neighbor by its id.
func (m *Manager) GetNeighbor(id identity.ID) (*Neighbor, error) {
	m.neighborsMutex.RLock()
	defer m.neighborsMutex.RUnlock()
	nbr, ok := m.neighbors[id]
	if !ok {
		return nil, ErrUnknownNeighbor
	}
	return nbr, nil
}

// DropNeighbor disconnects the neighbor with the given ID and the group.
func (m *Manager) DropNeighbor(id identity.ID, group NeighborsGroup) error {
	nbr, err := m.getNeighborWithGroup(id, group)
	if err != nil {
		return errors.WithStack(err)
	}
	nbr.close()
	return nil
}

// getNeighborWithGroup returns neighbor by ID and group.
func (m *Manager) getNeighborWithGroup(id identity.ID, group NeighborsGroup) (*Neighbor, error) {
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
	packet := &pb.Packet{Body: &pb.Packet_MessageRequest{MessageRequest: msgReq}}
	recipients := m.send(packet, to...)
	if m.messagesRateLimiter != nil {
		for _, nbr := range recipients {
			// Increase the limit by 2 for every message request to make rate limiter more forgiving during node sync.
			m.messagesRateLimiter.ExtendLimit(nbr.Peer, 2)
		}
	}
}

// SendMessage adds the given message the send queue of the neighbors.
// The actual send then happens asynchronously. If no peer is provided, it is send to all neighbors.
func (m *Manager) SendMessage(msgData []byte, to ...identity.ID) {
	msg := &pb.Message{Data: msgData}
	packet := &pb.Packet{Body: &pb.Packet_Message{Message: msg}}
	m.send(packet, to...)
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

func (m *Manager) send(packet *pb.Packet, to ...identity.ID) []*Neighbor {
	neighbors := m.getNeighborsByID(to)
	if len(neighbors) == 0 {
		neighbors = m.AllNeighbors()
	}

	for _, nbr := range neighbors {
		if err := nbr.ps.writePacket(packet); err != nil {
			m.log.Warnw("send error", "peer-id", nbr.ID(), "err", err)
			nbr.close()
		}
	}
	return neighbors
}

func (m *Manager) addNeighbor(ctx context.Context, p *peer.Peer, group NeighborsGroup,
	connectorFunc func(context.Context, *peer.Peer, []ConnectPeerOption) (*packetsStream, error),
	connectOpts []ConnectPeerOption,
) error {
	if p.ID() == m.local.ID() {
		return errors.WithStack(ErrLoopbackNeighbor)
	}
	m.stopMutex.RLock()
	defer m.stopMutex.RUnlock()
	if m.isStopped {
		return ErrNotRunning
	}
	if m.neighborExists(p.ID()) {
		return errors.WithStack(ErrDuplicateNeighbor)
	}

	ps, err := connectorFunc(ctx, p, connectOpts)
	if err != nil {
		return errors.WithStack(err)
	}

	// create and add the neighbor
	nbr := NewNeighbor(p, group, ps, m.log)
	if err := m.setNeighbor(nbr); err != nil {
		if resetErr := ps.Close(); resetErr != nil {
			err = errors.CombineErrors(err, resetErr)
		}
		return errors.WithStack(err)
	}
	nbr.disconnected.Attach(events.NewClosure(func() {
		m.deleteNeighbor(nbr)
		go m.NeighborsEvents(nbr.Group).NeighborRemoved.Trigger(nbr)
	}))
	nbr.packetReceived.Attach(events.NewClosure(func(packet *pb.Packet) {
		if err := m.handlePacket(packet, nbr); err != nil {
			nbr.log.Debugw("Can't handle packet", "err", err)
		}
	}))
	nbr.readLoop()
	nbr.log.Info("Connection established")
	m.neighborsEvents[group].NeighborAdded.Trigger(nbr)

	return nil
}

func (m *Manager) neighborExists(id identity.ID) bool {
	m.neighborsMutex.RLock()
	defer m.neighborsMutex.RUnlock()
	_, exists := m.neighbors[id]
	return exists
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
	m.neighbors[nbr.ID()] = nbr
	return nil
}

func (m *Manager) handlePacket(packet *pb.Packet, nbr *Neighbor) error {
	switch packetBody := packet.GetBody().(type) {
	case *pb.Packet_Message:
		if _, added := m.messageWorkerPool.TrySubmit(packetBody, nbr); !added {
			return fmt.Errorf("messageWorkerPool full: packet message discarded")
		}
	case *pb.Packet_MessageRequest:
		if _, added := m.messageRequestWorkerPool.TrySubmit(packetBody, nbr); !added {
			return fmt.Errorf("messageRequestWorkerPool full: message request discarded")
		}

	default:
		return errors.Newf("unsupported packet; packet=%+v, packetBody=%T-%+v", packet, packetBody, packetBody)
	}

	return nil
}

// MessageWorkerPoolStatus returns the name and the load of the workerpool.
func (m *Manager) MessageWorkerPoolStatus() (name string, load int) {
	return "messageWorkerPool", m.messageWorkerPool.GetPendingQueueSize()
}

// MessageRequestWorkerPoolStatus returns the name and the load of the workerpool.
func (m *Manager) MessageRequestWorkerPoolStatus() (name string, load int) {
	return "messageRequestWorkerPool", m.messageRequestWorkerPool.GetPendingQueueSize()
}

func (m *Manager) processMessagePacket(packetMsg *pb.Packet_Message, nbr *Neighbor) {
	if m.messagesRateLimiter != nil {
		m.messagesRateLimiter.Count(nbr.Peer)
	}
	m.events.MessageReceived.Trigger(&MessageReceivedEvent{Data: packetMsg.Message.GetData(), Peer: nbr.Peer})
}

func (m *Manager) processMessageRequestPacket(packetMsgReq *pb.Packet_MessageRequest, nbr *Neighbor) {
	if m.messageRequestsRateLimiter != nil {
		m.messageRequestsRateLimiter.Count(nbr.Peer)
	}
	msgID, _, err := tangle.MessageIDFromBytes(packetMsgReq.MessageRequest.GetId())
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
	packet := &pb.Packet{Body: &pb.Packet_Message{Message: &pb.Message{Data: msgBytes}}}
	if err := nbr.ps.writePacket(packet); err != nil {
		nbr.log.Warnw("Failed to send requested message back to the neighbor", "err", err)
		nbr.close()
	}
}
