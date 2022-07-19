package warpsync

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/p2p"
	"github.com/iotaledger/goshimmer/packages/tangle"
	wp "github.com/iotaledger/goshimmer/packages/warpsync/warpsyncproto"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"google.golang.org/protobuf/proto"
)

const (
	protocolID = "warpsync/0.0.1"
)

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(blockId tangle.BlockID) ([]byte, error)

// The Manager handles the connected neighbors.
type Manager struct {
	p2pManager *p2p.Manager

	Events *Events

	loadBlockFunc LoadBlockFunc
	log           *logger.Logger

	stopMutex sync.RWMutex
	isStopped bool

	concurrency int

	validationInProgress bool
	commitmentsChan      chan (*epoch.ECRecord)
}

// ManagerOption configures the Manager instance.
type ManagerOption func(m *Manager)

// NewManager creates a new Manager.
func NewManager(p2pManager *p2p.Manager, f LoadBlockFunc, log *logger.Logger, opts ...ManagerOption) *Manager {
	m := &Manager{
		p2pManager:    p2pManager,
		Events:        newEvents(),
		loadBlockFunc: f,
		log:           log,
	}

	m.p2pManager.RegisterProtocol(protocolID, &p2p.ProtocolHandler{
		PacketFactory:      warpsyncPacketFactory,
		NegotiationSend:    sendNegotiationMessage,
		NegotiationReceive: receiveNegotiationMessage,
		PacketHandler:      m.handlePacket,
	})

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithConcurrency allows to set how many epochs can be requested at once.
func WithConcurrency(concurrency int) ManagerOption {
	return func(m *Manager) {
		m.concurrency = concurrency
	}
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.stopMutex.Lock()
	defer m.stopMutex.Unlock()

	if m.isStopped {
		return
	}
	m.isStopped = true
	m.p2pManager.UnregisterProtocol(protocolID)
}

func (m *Manager) send(packet *wp.Packet, to ...identity.ID) []*p2p.Neighbor {
	neighbors := m.p2pManager.GetNeighborsByID(to)
	if len(neighbors) == 0 {
		neighbors = m.p2pManager.AllNeighbors()
	}

	for _, nbr := range neighbors {
		if err := nbr.GetStream(protocolID).WritePacket(packet); err != nil {
			m.log.Warnw("send error", "peer-id", nbr.ID(), "err", err)
			nbr.Close()
		}
	}
	return neighbors
}

func (m *Manager) handlePacket(nbr *p2p.Neighbor, packet proto.Message) error {
	gpPacket := packet.(*wp.Packet)
	switch packetBody := gpPacket.GetBody().(type) {
	case *wp.Packet_EpochBlocksRequest:
		if added := event.Loop.TrySubmit(func() { m.processEpochRequestPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("blockWorkerPool full: packet block discarded")
		}
	case *wp.Packet_EpochBlocks:
		if added := event.Loop.TrySubmit(func() { m.processEpochBlocksPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("blockRequestWorkerPool full: block request discarded")
		}
	case *wp.Packet_EpochCommitmentRequest:
		if added := event.Loop.TrySubmit(func() { m.processEpochRequestPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("blockWorkerPool full: packet block discarded")
		}
	case *wp.Packet_EpochCommitment:
		if added := event.Loop.TrySubmit(func() { m.processEpochBlocksPacket(packetBody, nbr) }); !added {
			return fmt.Errorf("blockRequestWorkerPool full: block request discarded")
		}
	default:
		return errors.Newf("unsupported packet; packet=%+v, packetBody=%T-%+v", gpPacket, packetBody, packetBody)
	}

	return nil
}

func warpsyncPacketFactory() proto.Message {
	return &wp.Packet{}
}

func sendNegotiationMessage(ps *p2p.PacketsStream) error {
	packet := &wp.Packet{Body: &wp.Packet_Negotiation{Negotiation: &wp.Negotiation{}}}
	return errors.WithStack(ps.WritePacket(packet))
}

func receiveNegotiationMessage(ps *p2p.PacketsStream) (err error) {
	packet := &wp.Packet{}
	if err := ps.ReadPacket(packet); err != nil {
		return errors.WithStack(err)
	}
	packetBody := packet.GetBody()
	if _, ok := packetBody.(*wp.Packet_Negotiation); !ok {
		return errors.Newf(
			"received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
			packet, packetBody, packetBody,
		)
	}
	return nil
}
