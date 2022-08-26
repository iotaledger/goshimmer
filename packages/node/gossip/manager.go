package gossip

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/app/ratelimiter"
	gp "github.com/iotaledger/goshimmer/packages/node/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
)

const (
	protocolID = "gossip/0.0.1"
)

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(blockId tangleold.BlockID) ([]byte, error)

// The Manager handles the connected neighbors.
type Manager struct {
	p2pManager *p2p.Manager

	Events *Events

	loadBlockFunc LoadBlockFunc
	log           *logger.Logger

	stopMutex sync.RWMutex
	isStopped bool

	blocksRateLimiter        *ratelimiter.PeerRateLimiter
	blockRequestsRateLimiter *ratelimiter.PeerRateLimiter

	pendingCount          atomic.Uint64
	requesterPendingCount atomic.Uint64
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
		PacketFactory:      gossipPacketFactory,
		NegotiationSend:    sendNegotiationMessage,
		NegotiationReceive: receiveNegotiationMessage,
		PacketHandler:      m.handlePacket,
	})

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithBlocksRateLimiter allows to set a PeerRateLimiter instance
// to be used as blocks rate limiter in the gossip manager.
func WithBlocksRateLimiter(prl *ratelimiter.PeerRateLimiter) ManagerOption {
	return func(m *Manager) {
		m.blocksRateLimiter = prl
	}
}

// BlocksRateLimiter returns the blocks rate limiter instance used in the gossip manager.
func (m *Manager) BlocksRateLimiter() *ratelimiter.PeerRateLimiter {
	return m.blocksRateLimiter
}

// WithBlockRequestsRateLimiter allows to set a PeerRateLimiter instance
// to be used as blocks requests rate limiter in the gossip manager.
func WithBlockRequestsRateLimiter(prl *ratelimiter.PeerRateLimiter) ManagerOption {
	return func(m *Manager) {
		m.blockRequestsRateLimiter = prl
	}
}

// BlockRequestsRateLimiter returns the block requests rate limiter instance used in the gossip manager.
func (m *Manager) BlockRequestsRateLimiter() *ratelimiter.PeerRateLimiter {
	return m.blockRequestsRateLimiter
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

// RequestBlock requests the block with the given id from the neighbors.
// If no peer is provided, all neighbors are queried.
func (m *Manager) RequestBlock(blockID []byte, to ...identity.ID) {
	blkReq := &gp.BlockRequest{Id: blockID}
	packet := &gp.Packet{Body: &gp.Packet_BlockRequest{BlockRequest: blkReq}}
	recipients := m.p2pManager.Send(packet, protocolID, to...)
	if m.blocksRateLimiter != nil {
		for _, nbr := range recipients {
			// Increase the limit by 2 for every block request to make rate limiter more forgiving during node sync.
			m.blocksRateLimiter.ExtendLimit(nbr.Peer, 2)
		}
	}
}

// SendBlock adds the given block the send queue of the neighbors.
// The actual send then happens asynchronously. If no peer is provided, it is send to all neighbors.
func (m *Manager) SendBlock(blkData []byte, to ...identity.ID) {
	blk := &gp.Block{Data: blkData}
	packet := &gp.Packet{Body: &gp.Packet_Block{Block: blk}}
	m.p2pManager.Send(packet, protocolID, to...)
}

func (m *Manager) handlePacket(nbr *p2p.Neighbor, packet proto.Message) error {
	gpPacket := packet.(*gp.Packet)
	switch packetBody := gpPacket.GetBody().(type) {
	case *gp.Packet_Block:
		if added := event.Loop.TrySubmit(func() { m.processBlockPacket(packetBody, nbr); m.pendingCount.Dec() }); !added {
			return fmt.Errorf("blockWorkerPool full: packet block discarded")
		}
		m.pendingCount.Inc()
	case *gp.Packet_BlockRequest:
		if added := event.Loop.TrySubmit(func() { m.processBlockRequestPacket(packetBody, nbr); m.requesterPendingCount.Dec() }); !added {
			return fmt.Errorf("blockRequestWorkerPool full: block request discarded")
		}
		m.requesterPendingCount.Inc()
	default:
		return errors.Newf("unsupported packet; packet=%+v, packetBody=%T-%+v", gpPacket, packetBody, packetBody)
	}

	return nil
}

// BlockWorkerPoolStatus returns the name and the load of the workerpool.
func (m *Manager) BlockWorkerPoolStatus() (name string, load uint64) {
	return "blockWorkerPool", m.pendingCount.Load()
}

// BlockRequestWorkerPoolStatus returns the name and the load of the workerpool.
func (m *Manager) BlockRequestWorkerPoolStatus() (name string, load uint64) {
	return "blockRequestWorkerPool", m.requesterPendingCount.Load()
}

func (m *Manager) processBlockPacket(packetBlk *gp.Packet_Block, nbr *p2p.Neighbor) {
	if m.blocksRateLimiter != nil {
		m.blocksRateLimiter.Count(nbr.Peer)
	}
	m.Events.BlockReceived.Trigger(&BlockReceivedEvent{Data: packetBlk.Block.GetData(), Peer: nbr.Peer})
}

func (m *Manager) processBlockRequestPacket(packetBlkReq *gp.Packet_BlockRequest, nbr *p2p.Neighbor) {
	if m.blockRequestsRateLimiter != nil {
		m.blockRequestsRateLimiter.Count(nbr.Peer)
	}
	var blkID tangleold.BlockID
	_, err := blkID.FromBytes(packetBlkReq.BlockRequest.GetId())
	if err != nil {
		m.log.Debugw("invalid block id:", "err", err)
		return
	}

	blkBytes, err := m.loadBlockFunc(blkID)
	if err != nil {
		m.log.Debugw("error loading block", "blk-id", blkID, "err", err)
		return
	}

	// send the loaded block directly to the neighbor
	packet := &gp.Packet{Body: &gp.Packet_Block{Block: &gp.Block{Data: blkBytes}}}
	if err := nbr.GetStream(protocolID).WritePacket(packet); err != nil {
		nbr.Log.Warnw("Failed to send requested block back to the neighbor", "err", err)
		nbr.Close()
	}
}

func gossipPacketFactory() proto.Message {
	return &gp.Packet{}
}

func sendNegotiationMessage(ps *p2p.PacketsStream) error {
	packet := &gp.Packet{Body: &gp.Packet_Negotiation{Negotiation: &gp.Negotiation{}}}
	return errors.WithStack(ps.WritePacket(packet))
}

func receiveNegotiationMessage(ps *p2p.PacketsStream) (err error) {
	packet := &gp.Packet{}
	if err := ps.ReadPacket(packet); err != nil {
		return errors.WithStack(err)
	}
	packetBody := packet.GetBody()
	if _, ok := packetBody.(*gp.Packet_Negotiation); !ok {
		return errors.Newf(
			"received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
			packet, packetBody, packetBody,
		)
	}
	return nil
}
