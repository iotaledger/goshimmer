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

	"github.com/iotaledger/goshimmer/packages/interfaces"
	gp "github.com/iotaledger/goshimmer/packages/network/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/goshimmer/packages/app/ratelimiter"
)

const (
	protocolID = "gossip/0.0.1"
)

// The Manager handles the connected neighbors.
type Manager struct {
	network interfaces.Network

	Events *Events

	log *logger.Logger

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
func NewManager(network interfaces.Network, log *logger.Logger, opts ...ManagerOption) *Manager {
	m := &Manager{
		network: network,
		Events:  NewEvents(),
		log:     log,
	}

	m.network.RegisterProtocol(protocolID, gossipPacketFactory, m.handlePacket)

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
	m.network.UnregisterProtocol(protocolID)
}

// RequestBlock requests the block with the given id from the neighbors.
// If no peer is provided, all neighbors are queried.
func (m *Manager) RequestBlock(blockID []byte, to ...identity.ID) {
	blkReq := &gp.BlockRequest{Id: blockID}
	packet := &gp.Packet{Body: &gp.Packet_BlockRequest{BlockRequest: blkReq}}
	recipients := m.network.Send(packet, protocolID, to...)
	if m.blocksRateLimiter != nil {
		for _, recipientID := range recipients {
			// Increase the limit by 2 for every block request to make rate limiter more forgiving during node sync.
			m.blocksRateLimiter.ExtendLimit(recipientID, 2)
		}
	}
}

// SendBlock adds the given block the send queue of the neighbors.
// The actual send then happens asynchronously. If no peer is provided, it is send to all neighbors.
func (m *Manager) SendBlock(blkData []byte, to ...identity.ID) {
	blk := &gp.Block{Data: blkData}
	packet := &gp.Packet{Body: &gp.Packet_Block{Block: blk}}
	m.network.Send(packet, protocolID, to...)
}

func (m *Manager) handlePacket(id identity.ID, packet proto.Message) error {
	gpPacket := packet.(*gp.Packet)
	switch packetBody := gpPacket.GetBody().(type) {
	case *gp.Packet_Block:
		if added := event.Loop.TrySubmit(func() { m.processBlockPacket(packetBody, id); m.pendingCount.Dec() }); !added {
			return fmt.Errorf("blockWorkerPool full: packet block discarded")
		}
		m.pendingCount.Inc()
	case *gp.Packet_BlockRequest:
		if added := event.Loop.TrySubmit(func() { m.processBlockRequestPacket(packetBody, id); m.requesterPendingCount.Dec() }); !added {
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

func (m *Manager) processBlockPacket(packetBlk *gp.Packet_Block, id identity.ID) {
	if m.blocksRateLimiter != nil {
		m.blocksRateLimiter.Count(id)
	}
	m.Events.BlockReceived.Trigger(&BlockReceivedEvent{
		Source: id,
		Data:   packetBlk.Block.GetData(),
	})
}

func (m *Manager) processBlockRequestPacket(packetBlkReq *gp.Packet_BlockRequest, id identity.ID) {
	if m.blockRequestsRateLimiter != nil {
		m.blockRequestsRateLimiter.Count(id)
	}
	var blkID models.BlockID
	_, err := blkID.FromBytes(packetBlkReq.BlockRequest.GetId())
	if err != nil {
		m.log.Debugw("invalid block id:", "err", err)
		return
	}

	m.Events.BlockRequestReceived.Trigger(&BlockRequestReceived{
		Source:  id,
		BlockID: blkID,
	})
}

func gossipPacketFactory() proto.Message {
	return &gp.Packet{}
}
