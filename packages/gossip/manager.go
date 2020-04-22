package gossip

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
)

const (
	maxPacketSize = 2048
)

var (
	ErrNeighborManagerNotRunning = errors.New("neighbor manager is not running")
	ErrNeighborAlreadyConnected  = errors.New("neighbor is already connected")
)

// LoadMessageFunc defines a function that returns the message for the given id.
type LoadMessageFunc func(messageId message.Id) ([]byte, error)

type Manager struct {
	local           *peer.Local
	loadMessageFunc LoadMessageFunc
	log             *zap.SugaredLogger

	wg sync.WaitGroup

	mu        sync.RWMutex
	srv       *server.TCP
	neighbors map[identity.ID]*Neighbor
	running   bool
}

func NewManager(local *peer.Local, f LoadMessageFunc, log *zap.SugaredLogger) *Manager {
	return &Manager{
		local:           local,
		loadMessageFunc: f,
		log:             log,
		srv:             nil,
		neighbors:       make(map[identity.ID]*Neighbor),
		running:         false,
	}
}

func (m *Manager) Start(srv *server.TCP) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.srv = srv
	m.running = true
}

// Close stops the manager and closes all established connections.
func (m *Manager) Close() {
	m.stop()
	m.wg.Wait()
}

func (m *Manager) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.running = false

	// close all neighbor connections
	for _, nbr := range m.neighbors {
		_ = nbr.Close()
	}
}

// AddOutbound tries to add a neighbor by connecting to that peer.
func (m *Manager) AddOutbound(p *peer.Peer) error {
	if p.ID() == m.local.ID() {
		return ErrLoopback
	}
	var srv *server.TCP
	m.mu.RLock()
	if m.srv == nil {
		m.mu.RUnlock()
		return ErrNotStarted
	}
	srv = m.srv
	m.mu.RUnlock()

	return m.addNeighbor(p, srv.DialPeer)
}

// AddInbound tries to add a neighbor by accepting an incoming connection from that peer.
func (m *Manager) AddInbound(p *peer.Peer) error {
	if p.ID() == m.local.ID() {
		return ErrLoopback
	}
	var srv *server.TCP
	m.mu.RLock()
	if m.srv == nil {
		m.mu.RUnlock()
		return ErrNotStarted
	}
	srv = m.srv
	m.mu.RUnlock()

	return m.addNeighbor(p, srv.AcceptPeer)
}

// NeighborRemoved disconnects the neighbor with the given ID.
func (m *Manager) DropNeighbor(id identity.ID) error {
	n, err := m.removeNeighbor(id)
	if err != nil {
		return err
	}
	err = n.Close()
	Events.NeighborRemoved.Trigger(n.Peer)
	return err
}

// RequestMessage requests the message with the given id from the neighbors.
// If no peer is provided, all neighbors are queried.
func (m *Manager) RequestMessage(messageId []byte, to ...identity.ID) {
	msgReq := &pb.MessageRequest{Id: messageId}
	m.log.Debugw("send packet", "type", msgReq.Type(), "to", to)
	m.send(marshal(msgReq), to...)
}

// SendMessage adds the given message the send queue of the neighbors.
// The actual send then happens asynchronously. If no peer is provided, it is send to all neighbors.
func (m *Manager) SendMessage(msgData []byte, to ...identity.ID) {
	msg := &pb.Message{Data: msgData}
	m.log.Debugw("send packet", "type", msg.Type(), "to", to)
	m.send(marshal(msg), to...)
}

func (m *Manager) GetAllNeighbors() []*Neighbor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Neighbor, 0, len(m.neighbors))
	for _, n := range m.neighbors {
		result = append(result, n)
	}
	return result
}

func (m *Manager) getNeighbors(ids ...identity.ID) []*Neighbor {
	if len(ids) > 0 {
		return m.getNeighborsById(ids)
	}
	return m.GetAllNeighbors()
}

func (m *Manager) getNeighborsById(ids []identity.ID) []*Neighbor {
	result := make([]*Neighbor, 0, len(ids))

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range ids {
		if n, ok := m.neighbors[id]; ok {
			result = append(result, n)
		}
	}
	return result
}

func (m *Manager) send(b []byte, to ...identity.ID) {
	neighbors := m.getNeighbors(to...)

	for _, nbr := range neighbors {
		if _, err := nbr.Write(b); err != nil {
			m.log.Warnw("send error", "err", err, "neighbor", nbr.Peer.Address())
		}
	}
}

func (m *Manager) addNeighbor(peer *peer.Peer, connectorFunc func(*peer.Peer) (net.Conn, error)) error {
	conn, err := connectorFunc(peer)
	if err != nil {
		Events.ConnectionFailed.Trigger(peer, err)
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		_ = conn.Close()
		Events.ConnectionFailed.Trigger(peer, ErrNeighborManagerNotRunning)
		return ErrClosed
	}
	if _, ok := m.neighbors[peer.ID()]; ok {
		_ = conn.Close()
		Events.ConnectionFailed.Trigger(peer, ErrNeighborAlreadyConnected)
		return ErrDuplicateNeighbor
	}

	// create and add the neighbor
	n := NewNeighbor(peer, conn, m.log)
	n.Events.Close.Attach(events.NewClosure(func() { _ = m.DropNeighbor(peer.ID()) }))
	n.Events.ReceiveMessage.Attach(events.NewClosure(func(data []byte) {
		if err := m.handlePacket(data, peer); err != nil {
			m.log.Debugw("error handling packet", "err", err)
		}
	}))

	m.neighbors[peer.ID()] = n
	n.Listen()
	Events.NeighborAdded.Trigger(n)

	return nil
}

func (m *Manager) removeNeighbor(id identity.ID) (*Neighbor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.neighbors[id]; !ok {
		return nil, ErrNotANeighbor
	}
	n := m.neighbors[id]
	delete(m.neighbors, id)
	return n, nil
}

func (m *Manager) handlePacket(data []byte, p *peer.Peer) error {
	// ignore empty packages
	if len(data) == 0 {
		return nil
	}

	switch pb.PacketType(data[0]) {

	case pb.PacketMessage:
		protoMsg := new(pb.Message)
		if err := proto.Unmarshal(data[1:], protoMsg); err != nil {
			return fmt.Errorf("invalid packet: %w", err)
		}
		m.log.Debugw("received packet", "type", protoMsg.Type(), "peer-id", p.ID())
		Events.MessageReceived.Trigger(&MessageReceivedEvent{Data: protoMsg.GetData(), Peer: p})

	case pb.PacketMessageRequest:
		protoMsgReq := new(pb.MessageRequest)
		if err := proto.Unmarshal(data[1:], protoMsgReq); err != nil {
			return fmt.Errorf("invalid packet: %w", err)
		}

		m.log.Debugw("received packet", "type", protoMsgReq.Type(), "peer-id", p.ID())
		msgId, err, _ := message.IdFromBytes(protoMsgReq.GetId())
		if err != nil {
			m.log.Debugw("couldn't compute message id from bytes", "peer-id", p.ID(), "err", err)
			return nil
		}

		msg, err := m.loadMessageFunc(msgId)
		if err != nil {
			m.log.Debugw("error getting message", "peer-id", p.ID(), "msg-id", msgId, "err", err)
			return nil
		}

		m.SendMessage(msg, p.ID())
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
