package gossip

import (
	"io"
	"net"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/transport"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	maxConnectionAttempts = 3
	maxPacketSize         = 2048
)

type GetTransaction func(txHash []byte) ([]byte, error)

type Manager struct {
	trans          *transport.TCP
	log            *zap.SugaredLogger
	getTransaction GetTransaction

	wg sync.WaitGroup

	mu        sync.RWMutex
	neighbors map[peer.ID]*neighbor
	running   bool
}

type neighbor struct {
	peer *peer.Peer
	conn *transport.Connection
}

func NewManager(t *transport.TCP, log *zap.SugaredLogger, f GetTransaction) *Manager {
	m := &Manager{
		trans:          t,
		log:            log,
		getTransaction: f,
		neighbors:      make(map[peer.ID]*neighbor),
	}
	m.running = true
	return m
}

// Close stops the manager and closes all established connections.
func (m *Manager) Close() {
	m.mu.Lock()
	m.running = false
	// close all connections
	for _, n := range m.neighbors {
		_ = n.conn.Close()
	}
	m.mu.Unlock()

	m.wg.Wait()
}

// AddOutbound tries to add a neighbor by connecting to that peer.
func (m *Manager) AddOutbound(p *peer.Peer) error {
	return m.addNeighbor(p, m.trans.DialPeer)
}

// AddInbound tries to add a neighbor by accepting an incoming connection from that peer.
func (m *Manager) AddInbound(p *peer.Peer) error {
	return m.addNeighbor(p, m.trans.AcceptPeer)
}

// NeighborDropped disconnects the neighbor with the given ID.
func (m *Manager) DropNeighbor(id peer.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.neighbors[id]; !ok {
		return ErrNotANeighbor
	}
	n := m.neighbors[id]
	delete(m.neighbors, id)
	disconnect(n.conn)

	return nil
}

// RequestTransaction requests the transaction with the given hash from the neighbors.
// If no peer is provided, all neighbors are queried.
func (m *Manager) RequestTransaction(txHash []byte, to ...peer.ID) {
	req := &pb.TransactionRequest{
		Hash: txHash,
	}
	m.send(marshal(req), to...)
}

// SendTransaction sends the given transaction data to the neighbors.
// If no peer is provided, it is send to all neighbors.
func (m *Manager) SendTransaction(txData []byte, to ...peer.ID) {
	tx := &pb.Transaction{
		Body: txData,
	}
	m.send(marshal(tx), to...)
}

func (m *Manager) getNeighbors(ids ...peer.ID) []*neighbor {
	if len(ids) > 0 {
		return m.getNeighborsById(ids)
	}
	return m.getAllNeighbors()
}

func (m *Manager) getAllNeighbors() []*neighbor {
	m.mu.RLock()
	result := make([]*neighbor, 0, len(m.neighbors))
	for _, n := range m.neighbors {
		result = append(result, n)
	}
	m.mu.RUnlock()

	return result
}

func (m *Manager) getNeighborsById(ids []peer.ID) []*neighbor {
	result := make([]*neighbor, 0, len(ids))

	m.mu.RLock()
	for _, id := range ids {
		if n, ok := m.neighbors[id]; ok {
			result = append(result, n)
		}
	}
	m.mu.RUnlock()

	return result
}

func (m *Manager) send(msg []byte, to ...peer.ID) {
	if l := len(msg); l > maxPacketSize {
		m.log.Errorw("message too large", "len", l, "max", maxPacketSize)
	}
	neighbors := m.getNeighbors(to...)

	for _, nbr := range neighbors {
		m.log.Debugw("Sending", "to", nbr.peer.ID(), "msg", msg)
		_, err := nbr.conn.Write(msg)
		if err != nil {
			m.log.Debugw("send error", "err", err)
		}
	}
}

func (m *Manager) addNeighbor(peer *peer.Peer, connectorFunc func(*peer.Peer) (*transport.Connection, error)) error {
	var (
		err  error
		conn *transport.Connection
	)
	for i := 0; i < maxConnectionAttempts; i++ {
		conn, err = connectorFunc(peer)
		if err == nil {
			break
		}
	}

	// could not add neighbor
	if err != nil {
		m.log.Debugw("addNeighbor failed", "peer", peer.ID(), "err", err)
		Events.NeighborDropped.Trigger(&NeighborDroppedEvent{Peer: peer})
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		disconnect(conn)
		return ErrClosed
	}
	if _, ok := m.neighbors[peer.ID()]; ok {
		disconnect(conn)
		return ErrDuplicateNeighbor
	}

	// add the neighbor
	n := &neighbor{
		peer: peer,
		conn: conn,
	}
	m.neighbors[peer.ID()] = n
	go m.readLoop(n)

	return nil
}

func (m *Manager) readLoop(nbr *neighbor) {
	m.wg.Add(1)
	defer m.wg.Done()

	// create a buffer for the packages
	b := make([]byte, maxPacketSize)

	for {
		n, err := nbr.conn.Read(b)
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			// ignore temporary read errors.
			m.log.Debugw("temporary read error", "err", err)
			continue
		} else if err != nil {
			// return from the loop on all other errors
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				m.log.Warnw("read error", "err", err)
			}

			m.log.Debug("connection closed", "id", nbr.peer.ID(), "addr", nbr.conn.RemoteAddr().String())
			_ = nbr.conn.Close() // just make sure that the connection is closed as fast as possible
			_ = m.DropNeighbor(nbr.peer.ID())
			return
		}

		if err := m.handlePacket(b[:n], nbr); err != nil {
			m.log.Warnw("failed to handle packet", "id", nbr.peer.ID(), "err", err)
		}
	}
}

func (m *Manager) handlePacket(data []byte, n *neighbor) error {
	switch pb.MType(data[0]) {

	// Incoming Transaction
	case pb.MTransaction:
		msg := new(pb.Transaction)
		if err := proto.Unmarshal(data[1:], msg); err != nil {
			return errors.Wrap(err, "invalid message")
		}
		m.log.Debugw("Received Transaction", "data", msg.GetBody())
		Events.TransactionReceived.Trigger(&TransactionReceivedEvent{Body: msg.GetBody(), Peer: n.peer})

	// Incoming Transaction request
	case pb.MTransactionRequest:
		msg := new(pb.TransactionRequest)
		if err := proto.Unmarshal(data[1:], msg); err != nil {
			return errors.Wrap(err, "invalid message")
		}
		m.log.Debugw("Received Tx Req", "data", msg.GetHash())
		// do something
		tx, err := m.getTransaction(msg.GetHash())
		if err != nil {
			m.log.Debugw("Tx not available", "tx", msg.GetHash())
		} else {
			m.log.Debugw("Tx found", "tx", tx)
			m.SendTransaction(tx, n.peer.ID())
		}

	default:
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

func disconnect(conn *transport.Connection) {
	_ = conn.Close()
	Events.NeighborDropped.Trigger(&NeighborDroppedEvent{Peer: conn.Peer()})
}
