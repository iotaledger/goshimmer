package gossip

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	pb "github.com/iotaledger/goshimmer/packages/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/p2p"
	"github.com/iotaledger/goshimmer/packages/ratelimiter"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/atomic"
)

const (
	defaultConnectionTimeout = 5 * time.Second // timeout after which the connection must be established.
	protocolID               = "gossip/0.0.1"
	ioTimeout                = 4 * time.Second
)

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(blockId tangle.BlockID) ([]byte, error)

// The Manager handles the connected neighbors.
type Manager struct {
	local      *peer.Local
	Libp2pHost host.Host

	Events *Events

	acceptWG    sync.WaitGroup
	acceptMutex sync.RWMutex

	loadBlockFunc   LoadBlockFunc
	log             *logger.Logger
	neighborsEvents map[p2p.NeighborsGroup]*p2p.NeighborsEvents

	stopMutex sync.RWMutex
	isStopped bool

	neighbors      map[identity.ID]*p2p.Neighbor
	neighborsMutex sync.RWMutex

	blocksRateLimiter        *ratelimiter.PeerRateLimiter
	blockRequestsRateLimiter *ratelimiter.PeerRateLimiter

	pendingCount          atomic.Uint64
	requesterPendingCount atomic.Uint64

	*p2p.Manager
}

// ManagerOption configures the Manager instance.
type ManagerOption func(m *Manager)

// NewManager creates a new Manager.
func NewManager(libp2pHost host.Host, local *peer.Local, f LoadBlockFunc, log *logger.Logger, opts ...ManagerOption,
) *Manager {
	m := &Manager{
		Libp2pHost:    libp2pHost,
		Events:        newEvents(),
		local:         local,
		loadBlockFunc: f,
		log:           log,
	}

	m.Libp2pHost.SetStreamHandler(protocolID, m.gossipStreamHandler)
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
	m.Libp2pHost.RemoveStreamHandler(protocolID)
}

func (m *Manager) newGossipPacketStream(ctx context.Context, libp2pID libp2ppeer.ID) (*p2p.PacketsStream, error) {
	stream, err := m.Libp2pHost.NewStream(ctx, libp2pID, protocolID)
	if err != nil {
		return nil, err
	}
	ps := p2p.NewPacketsStream(stream)
	if err := sendNegotiationBlock(ps); err != nil {
		err = errors.Wrap(err, "failed to send negotiation block")
		err = errors.CombineErrors(err, stream.Close())
		return nil, err
	}
	return ps, nil
}

func (m *Manager) gossipStreamHandler(stream network.Stream) {
	ps := p2p.NewPacketsStream(stream)
	if err := receiveNegotiationBlock(ps); err != nil {
		m.log.Warnw("Failed to receive negotiation block", "err", err)
		m.CloseStream(stream)
		return
	}
	am := m.MatchNewStream(stream)
	if am != nil {
		am.StreamCh <- ps
	} else {
		// close the connection if not matched
		m.log.Debugw("unexpected connection", "addr", stream.Conn().RemoteMultiaddr(),
			"id", stream.Conn().RemotePeer())
		m.CloseStream(stream)
	}
}

// RequestBlock requests the block with the given id from the neighbors.
// If no peer is provided, all neighbors are queried.
func (m *Manager) RequestBlock(blockID []byte, to ...identity.ID) {
	blkReq := &pb.BlockRequest{Id: blockID}
	packet := &pb.Packet{Body: &pb.Packet_BlockRequest{BlockRequest: blkReq}}
	recipients := m.send(packet, to...)
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
	blk := &pb.Block{Data: blkData}
	packet := &pb.Packet{Body: &pb.Packet_Block{Block: blk}}
	m.send(packet, to...)
}

func (m *Manager) send(packet *pb.Packet, to ...identity.ID) []*p2p.Neighbor {
	neighbors := m.GetNeighborsByID(to)
	if len(neighbors) == 0 {
		neighbors = m.AllNeighbors()
	}

	for _, nbr := range neighbors {
		if err := nbr.Ps.WritePacket(packet); err != nil {
			m.log.Warnw("send error", "peer-id", nbr.ID(), "err", err)
			nbr.Close()
		}
	}
	return neighbors
}

func (m *Manager) handlePacket(packet *pb.Packet, nbr *p2p.Neighbor) error {
	switch packetBody := packet.GetBody().(type) {
	case *pb.Packet_Block:
		if added := event.Loop.TrySubmit(func() { m.processBlockPacket(packetBody, nbr); m.pendingCount.Dec() }); !added {
			return fmt.Errorf("blockWorkerPool full: packet block discarded")
		}
		m.pendingCount.Inc()
	case *pb.Packet_BlockRequest:
		if added := event.Loop.TrySubmit(func() { m.processBlockRequestPacket(packetBody, nbr); m.requesterPendingCount.Dec() }); !added {
			return fmt.Errorf("blockRequestWorkerPool full: block request discarded")
		}
		m.requesterPendingCount.Inc()
	default:
		return errors.Newf("unsupported packet; packet=%+v, packetBody=%T-%+v", packet, packetBody, packetBody)
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

func (m *Manager) processBlockPacket(packetBlk *pb.Packet_Block, nbr *p2p.Neighbor) {
	if m.blocksRateLimiter != nil {
		m.blocksRateLimiter.Count(nbr.Peer)
	}
	m.Events.BlockReceived.Trigger(&BlockReceivedEvent{Data: packetBlk.Block.GetData(), Peer: nbr.Peer})
}

func (m *Manager) processBlockRequestPacket(packetBlkReq *pb.Packet_BlockRequest, nbr *p2p.Neighbor) {
	if m.blockRequestsRateLimiter != nil {
		m.blockRequestsRateLimiter.Count(nbr.Peer)
	}
	var blkID tangle.BlockID
	_, err := blkID.Decode(packetBlkReq.BlockRequest.GetId())
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
	packet := &pb.Packet{Body: &pb.Packet_Block{Block: &pb.Block{Data: blkBytes}}}
	if err := nbr.Ps.WritePacket(packet); err != nil {
		nbr.Log.Warnw("Failed to send requested block back to the neighbor", "err", err)
		nbr.Close()
	}
}

func sendNegotiationBlock(ps *p2p.PacketsStream) error {
	packet := &pb.Packet{Body: &pb.Packet_Negotiation{Negotiation: &pb.Negotiation{}}}
	return errors.WithStack(ps.WritePacket(packet))
}

func receiveNegotiationBlock(ps *p2p.PacketsStream) (err error) {
	packet := &pb.Packet{}
	if err := ps.ReadPacket(packet); err != nil {
		return errors.WithStack(err)
	}
	packetBody := packet.GetBody()
	if _, ok := packetBody.(*pb.Packet_Negotiation); !ok {
		return errors.Newf(
			"received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
			packet, packetBody, packetBody,
		)
	}
	return nil
}
