package gossip

import (
	"fmt"
	"net"
	"runtime"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/workerpool"
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

// NeighborsGroup is an enum type for various neighbors groups like auto/manual.
type NeighborsGroup int8

const (
	// NeighborsGroupAuto represents a neighbors group that is managed automatically.
	NeighborsGroupAuto NeighborsGroup = iota
	// NeighborsGroupManual represents a neighbors group that is managed manually.
	NeighborsGroupManual
)

type neighborWithGroup struct {
	*Neighbor
	group NeighborsGroup
}

// The Manager handles the connected neighbors.
type Manager struct {
	local           *peer.Local
	loadMessageFunc LoadMessageFunc
	log             *logger.Logger
	events          Events
	neighborsEvents map[NeighborsGroup]NeighborsEvents

	wg sync.WaitGroup

	mu        sync.RWMutex
	srv       *server.TCP
	neighbors map[identity.ID]*neighborWithGroup

	// messageWorkerPool defines a worker pool where all incoming messages are processed.
	messageWorkerPool *workerpool.WorkerPool

	messageRequestWorkerPool *workerpool.WorkerPool
}

// NewManager creates a new Manager.
func NewManager(local *peer.Local, f LoadMessageFunc, log *logger.Logger) *Manager {
	m := &Manager{
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
		srv:       nil,
		neighbors: make(map[identity.ID]*neighborWithGroup),
	}

	m.messageWorkerPool = workerpool.New(func(task workerpool.Task) {
		m.processPacketMessage(task.Param(0).([]byte), task.Param(1).(*Neighbor))

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))

	m.messageRequestWorkerPool = workerpool.New(func(task workerpool.Task) {
		m.processMessageRequest(task.Param(0).([]byte), task.Param(1).(*Neighbor))

		task.Return(nil)
	}, workerpool.WorkerCount(messageRequestWorkerCount), workerpool.QueueSize(messageRequestWorkerQueueSize))

	return m
}

// Start starts the manager for the given TCP server.
func (m *Manager) Start(srv *server.TCP) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.srv = srv

	m.messageWorkerPool.Start()
	m.messageRequestWorkerPool.Start()
}

// Close stops the manager and closes all established connections.
func (m *Manager) Close() {
	m.stop()
	m.wg.Wait()

	m.messageWorkerPool.Stop()
	m.messageRequestWorkerPool.Stop()
}

// Events returns the events related to the gossip protocol.
func (m *Manager) Events() Events {
	return m.events
}

// NeighborsEvents returns the events related to the gossip protocol.
func (m *Manager) NeighborsEvents(group NeighborsGroup) NeighborsEvents {
	return m.neighborsEvents[group]
}

func (m *Manager) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.srv = nil

	// close all neighbor connections
	for _, nbr := range m.neighbors {
		_ = nbr.Close()
	}
}

// AddOutbound tries to add a neighbor by connecting to that peer.
func (m *Manager) AddOutbound(p *peer.Peer, group NeighborsGroup) error {
	return m.addNeighbor(p, group, m.srv.DialPeer)
}

// AddInbound tries to add a neighbor by accepting an incoming connection from that peer.
func (m *Manager) AddInbound(p *peer.Peer, group NeighborsGroup) error {
	return m.addNeighbor(p, group, m.srv.AcceptPeer)
}

// DropNeighbor disconnects the neighbor with the given ID.
func (m *Manager) DropNeighbor(id identity.ID, group NeighborsGroup) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.neighbors[id]
	if !ok || n.group != group {
		return ErrUnknownNeighbor
	}
	delete(m.neighbors, id)

	return n.Close()
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
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Neighbor, 0, len(m.neighbors))
	for _, n := range m.neighbors {
		result = append(result, n.Neighbor)
	}
	return result
}

func (m *Manager) getNeighbors(ids ...identity.ID) []*Neighbor {
	if len(ids) > 0 {
		return m.getNeighborsByID(ids)
	}
	return m.AllNeighbors()
}

func (m *Manager) getNeighborsByID(ids []identity.ID) []*Neighbor {
	result := make([]*Neighbor, 0, len(ids))

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, id := range ids {
		if n, ok := m.neighbors[id]; ok {
			result = append(result, n.Neighbor)
		}
	}
	return result
}

func (m *Manager) send(b []byte, to ...identity.ID) {
	neighbors := m.getNeighbors(to...)

	for _, nbr := range neighbors {
		if _, err := nbr.Write(b); err != nil {
			m.log.Warnw("send error", "peer-id", nbr.ID(), "err", err)
		}
	}
}

func (m *Manager) addNeighbor(p *peer.Peer, group NeighborsGroup, connectorFunc func(*peer.Peer) (net.Conn, error),
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p.ID() == m.local.ID() {
		return ErrLoopbackNeighbor
	}
	if m.srv == nil {
		return ErrNotRunning
	}

	conn, err := connectorFunc(p)
	if err != nil {
		m.neighborsEvents[group].ConnectionFailed.Trigger(p, err)
		return err
	}

	if _, ok := m.neighbors[p.ID()]; ok {
		_ = conn.Close()
		m.neighborsEvents[group].ConnectionFailed.Trigger(p, ErrDuplicateNeighbor)
		return ErrDuplicateNeighbor
	}

	// create and add the neighbor
	nbr := NewNeighbor(p, conn, m.log)
	nbr.Events.Close.Attach(events.NewClosure(func() {
		// assure that the neighbor is removed and notify
		_ = m.DropNeighbor(p.ID(), group)
		m.neighborsEvents[group].NeighborRemoved.Trigger(nbr)
	}))
	nbr.Events.ReceiveMessage.Attach(events.NewClosure(func(data []byte) {
		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)
		if err := m.handlePacket(dataCopy, nbr); err != nil {
			m.log.Debugw("error handling packet", "err", err)
		}
	}))

	m.neighbors[p.ID()] = &neighborWithGroup{Neighbor: nbr, group: group}
	nbr.Listen()
	m.neighborsEvents[group].NeighborAdded.Trigger(nbr)

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
	}
	m.events.MessageReceived.Trigger(&MessageReceivedEvent{Data: packet.GetData(), Peer: nbr.Peer})
}

func (m *Manager) processMessageRequest(data []byte, nbr *Neighbor) {
	packet := new(pb.MessageRequest)
	if err := proto.Unmarshal(data[1:], packet); err != nil {
		m.log.Debugw("invalid packet", "err", err)
	}

	msgID, _, err := tangle.MessageIDFromBytes(packet.GetId())
	if err != nil {
		m.log.Debugw("invalid message id:", "err", err)
	}

	msgBytes, err := m.loadMessageFunc(msgID)
	if err != nil {
		m.log.Debugw("error loading message", "msg-id", msgID, "err", err)
	}

	// send the loaded message directly to the neighbor
	_, _ = nbr.Write(marshal(&pb.Message{Data: msgBytes}))
}
