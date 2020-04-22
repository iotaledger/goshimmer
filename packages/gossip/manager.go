package gossip

import (
	"fmt"
	"net"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/zap"
)

const (
	maxPacketSize = 2048
)

// GetTransaction defines a function that returns the transaction data with the given hash.
type GetTransaction func(transactionId message.Id) ([]byte, error)

type Manager struct {
	local          *peer.Local
	getTransaction GetTransaction
	log            *zap.SugaredLogger

	wg sync.WaitGroup

	mu        sync.Mutex
	srv       *server.TCP
	neighbors map[identity.ID]*Neighbor
}

func NewManager(local *peer.Local, f GetTransaction, log *zap.SugaredLogger) *Manager {
	return &Manager{
		local:          local,
		getTransaction: f,
		log:            log,
		srv:            nil,
		neighbors:      make(map[identity.ID]*Neighbor),
	}
}

func (m *Manager) Start(srv *server.TCP) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.srv = srv
}

// Close stops the manager and closes all established connections.
func (m *Manager) Close() {
	m.stop()
	m.wg.Wait()
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
func (m *Manager) AddOutbound(p *peer.Peer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p.ID() == m.local.ID() {
		return ErrLoopback
	}
	if m.srv == nil {
		return ErrNotRunning
	}
	return m.addNeighbor(p, m.srv.DialPeer)
}

// AddInbound tries to add a neighbor by accepting an incoming connection from that peer.
func (m *Manager) AddInbound(p *peer.Peer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p.ID() == m.local.ID() {
		return ErrLoopback
	}
	if m.srv == nil {
		return ErrNotRunning
	}
	return m.addNeighbor(p, m.srv.AcceptPeer)
}

// DropNeighbor disconnects the neighbor with the given ID.
func (m *Manager) DropNeighbor(id identity.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.neighbors[id]; !ok {
		return ErrNotANeighbor
	}
	n := m.neighbors[id]
	delete(m.neighbors, id)

	return n.Close()
}

// RequestTransaction requests the transaction with the given hash from the neighbors.
// If no peer is provided, all neighbors are queried.
func (m *Manager) RequestTransaction(txHash []byte, to ...identity.ID) {
	req := &pb.TransactionRequest{
		Hash: txHash,
	}
	m.log.Debugw("send message", "type", "TRANSACTION_REQUEST", "to", to)
	m.send(marshal(req), to...)
}

// SendTransaction adds the given transaction data to the send queue of the neighbors.
// The actual send then happens asynchronously. If no peer is provided, it is send to all neighbors.
func (m *Manager) SendTransaction(txData []byte, to ...identity.ID) {
	tx := &pb.Transaction{
		Data: txData,
	}
	m.log.Debugw("send message", "type", "TRANSACTION", "to", to)
	m.send(marshal(tx), to...)
}

func (m *Manager) GetAllNeighbors() []*Neighbor {
	m.mu.Lock()
	defer m.mu.Unlock()

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

	m.mu.Lock()
	defer m.mu.Unlock()

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

	if _, ok := m.neighbors[peer.ID()]; ok {
		_ = conn.Close()
		Events.ConnectionFailed.Trigger(peer, ErrDuplicateNeighbor)
		return ErrDuplicateNeighbor
	}

	// create and add the neighbor
	n := NewNeighbor(peer, conn, m.log)
	n.Events.Close.Attach(events.NewClosure(func() {
		// assure that the neighbor is removed and notify
		_ = m.DropNeighbor(peer.ID())
		Events.NeighborRemoved.Trigger(peer)
	}))
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

func (m *Manager) handlePacket(data []byte, p *peer.Peer) error {
	// ignore empty packages
	if len(data) == 0 {
		return nil
	}

	switch pb.MType(data[0]) {

	// Incoming Message
	case pb.MTransaction:
		msg := new(pb.Transaction)
		if err := proto.Unmarshal(data[1:], msg); err != nil {
			return fmt.Errorf("invalid packet: %w", err)
		}
		m.log.Debugw("received message", "type", "TRANSACTION", "id", p.ID())
		Events.TransactionReceived.Trigger(&TransactionReceivedEvent{Data: msg.GetData(), Peer: p})

	// Incoming Message request
	case pb.MTransactionRequest:

		msg := new(pb.TransactionRequest)
		if err := proto.Unmarshal(data[1:], msg); err != nil {
			return fmt.Errorf("invalid packet: %w", err)
		}
		m.log.Debugw("received message", "type", "TRANSACTION_REQUEST", "id", p.ID())
		// do something
		txId, err, _ := message.IdFromBytes(msg.GetHash())
		if err != nil {
			m.log.Debugw("error getting transaction", "hash", msg.GetHash(), "err", err)
		}
		tx, err := m.getTransaction(txId)
		if err != nil {
			m.log.Debugw("error getting transaction", "hash", msg.GetHash(), "err", err)
		} else {
			m.SendTransaction(tx, p.ID())
		}

	default:
		return ErrInvalidPacket
	}

	return nil
}

func marshal(msg pb.Message) []byte {
	mType := msg.Type()
	if mType > 0xFF {
		panic("invalid message")
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		panic("invalid message")
	}
	return append([]byte{byte(mType)}, data...)
}
