package gossip

import (
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/goshimmer/packages/gossip/neighbor"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/transport"
	"github.com/iotaledger/hive.go/events"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	maxAttempts = 3
)

var (
	Event Events
)

type GetTransaction func(txHash []byte) ([]byte, error)

type Manager struct {
	neighborhood   *neighbor.NeighborMap
	trans          *transport.TransportTCP
	log            *zap.SugaredLogger
	getTransaction GetTransaction
	Events         Events
}

func NewManager(t *transport.TransportTCP, log *zap.SugaredLogger, f GetTransaction) *Manager {
	mgr := &Manager{
		neighborhood:   neighbor.NewMap(),
		trans:          t,
		log:            log,
		getTransaction: f,
		Events: Events{
			NewTransaction: events.NewEvent(newTransaction),
			DropNeighbor:   events.NewEvent(dropNeighbor)},
	}
	Event = mgr.Events
	return mgr
}

func (m *Manager) AddOutbound(p *peer.Peer) error {
	return m.addNeighbor(p, m.trans.DialPeer)
}

func (m *Manager) AddInbound(p *peer.Peer) error {
	return m.addNeighbor(p, m.trans.AcceptPeer)
}

func (m *Manager) DropNeighbor(id peer.ID) {
	m.deleteNeighbor(id)
}

func (m *Manager) RequestTransaction(data []byte, to ...*neighbor.Neighbor) {
	req := &pb.TransactionRequest{}
	err := proto.Unmarshal(data, req)
	if err != nil {
		m.log.Warnw("Data to send is not a Transaction Request", "err", err)
	}
	msg := marshal(req)

	m.send(msg, to...)
}

func (m *Manager) Send(data []byte, to ...*neighbor.Neighbor) {
	tx := &pb.Transaction{}
	err := proto.Unmarshal(data, tx)
	if err != nil {
		m.log.Warnw("Data to send is not a Transaction", "err", err)
	}
	msg := marshal(tx)

	m.send(msg, to...)
}

func (m *Manager) send(msg []byte, to ...*neighbor.Neighbor) {
	neighbors := m.neighborhood.GetSlice()
	if to != nil {
		neighbors = to
	}

	for _, neighbor := range neighbors {
		m.log.Debugw("Sending", "to", neighbor.Peer.ID().String(), "msg", msg)
		err := neighbor.Conn.Write(msg)
		if err != nil {
			m.log.Debugw("send error", "err", err)
		}
	}
}

func (m *Manager) addNeighbor(peer *peer.Peer, handshake func(*peer.Peer) (*transport.Connection, error)) error {
	if _, ok := m.neighborhood.Load(peer.ID().String()); ok {
		return errors.New("Neighbor already added")
	}

	var err error
	var conn *transport.Connection
	i := 0
	for i = 0; i < maxAttempts; i++ {
		conn, err = handshake(peer)
		if err != nil {
			m.log.Warnw("Connection attempt failed", "attempt", i+1)
		} else {
			break
		}
	}
	if i == maxAttempts {
		m.log.Warnw("Connection failed to", "peer", peer.ID().String())
		m.Events.DropNeighbor.Trigger(&DropNeighborEvent{Peer: peer})
		return err
	}

	// add the new neighbor
	neighbor := neighbor.New(peer, conn)
	m.neighborhood.Store(peer.ID().String(), neighbor)

	// start listener for the new neighbor
	go m.readLoop(neighbor)

	return nil
}

func (m *Manager) deleteNeighbor(id peer.ID) {
	m.log.Debugw("Deleting neighbor", "neighbor", id.String())

	p, ok := m.neighborhood.Delete(id.String())
	if ok {
		m.Events.DropNeighbor.Trigger(&DropNeighborEvent{Peer: p.Peer})
	}
}

func (m *Manager) readLoop(neighbor *neighbor.Neighbor) {
	for {
		data, err := neighbor.Conn.Read()
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			// ignore temporary read errors.
			//m.log.Debugw("temporary read error", "err", err)
			continue
		} else if err != nil {
			// return from the loop on all other errors
			m.log.Debugw("reading stopped")
			m.deleteNeighbor(neighbor.Peer.ID())

			return
		}
		if err := m.handlePacket(data, neighbor); err != nil {
			m.log.Warnw("failed to handle packet", "from", neighbor.Peer.ID().String(), "err", err)
		}
	}
}

func (m *Manager) handlePacket(data []byte, neighbor *neighbor.Neighbor) error {
	switch pb.MType(data[0]) {

	// Incoming Transaction
	case pb.MTransaction:
		msg := new(pb.Transaction)
		if err := proto.Unmarshal(data[1:], msg); err != nil {
			return errors.Wrap(err, "invalid message")
		}
		m.log.Debugw("Received Transaction", "data", msg.GetBody())
		m.Events.NewTransaction.Trigger(&NewTransactionEvent{Body: msg.GetBody(), Peer: neighbor.Peer})

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
			m.Send(tx, neighbor)
		}

	default:
		return nil
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
