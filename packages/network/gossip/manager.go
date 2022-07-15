package gossip

import (
	"context"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/host"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/tangle"
	pb "github.com/iotaledger/goshimmer/packages/network/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/network/ratelimiter"
)

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(blockId tangle.BlockID) ([]byte, error)

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

	Events *Events

	acceptWG    sync.WaitGroup
	acceptMutex sync.RWMutex
	acceptMap   map[libp2ppeer.ID]*acceptMatcher

	loadBlockFunc   LoadBlockFunc
	log             *logger.Logger
	neighborsEvents map[NeighborsGroup]*NeighborsEvents

	stopMutex sync.RWMutex
	isStopped bool

	neighbors      map[identity.ID]*Neighbor
	neighborsMutex sync.RWMutex

	blocksRateLimiter        *ratelimiter.PeerRateLimiter
	blockRequestsRateLimiter *ratelimiter.PeerRateLimiter

	pendingCount          atomic.Uint64
	requesterPendingCount atomic.Uint64
}

// ManagerOption configures the Manager instance.
type ManagerOption func(m *Manager)

// NewManager creates a new Manager.
func NewManager(libp2pHost host.Host, local *peer.Local, f LoadBlockFunc, log *logger.Logger, opts ...ManagerOption,
) *Manager {
	m := &Manager{
		Libp2pHost:    libp2pHost,
		Events:        newEvents(),
		acceptMap:     map[libp2ppeer.ID]*acceptMatcher{},
		local:         local,
		loadBlockFunc: f,
		log:           log,
		neighborsEvents: map[NeighborsGroup]*NeighborsEvents{
			NeighborsGroupAuto:   NewNeighborsEvents(),
			NeighborsGroupManual: NewNeighborsEvents(),
		},
		neighbors: map[identity.ID]*Neighbor{},
	}

	m.Libp2pHost.SetStreamHandler(protocolID, m.streamHandler)
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
	m.dropAllNeighbors()
}

func (m *Manager) dropAllNeighbors() {
	neighborsList := m.AllNeighbors()
	for _, nbr := range neighborsList {
		nbr.close()
	}
}

// NeighborsEvents returns the events related to the gossip protocol.
func (m *Manager) NeighborsEvents(group NeighborsGroup) *NeighborsEvents {
	return m.neighborsEvents[group]
}

// AddOutbound tries to add a neighbor by connecting to that peer.
func (m *Manager) AddOutbound(ctx context.Context, p *peer.Peer, group NeighborsGroup,
	connectOpts ...ConnectPeerOption,
) error {
	return m.addNeighbor(ctx, p, group, m.dialPeer, connectOpts)
}

// AddInbound tries to add a neighbor by accepting an incoming connection from that peer.
func (m *Manager) AddInbound(ctx context.Context, p *peer.Peer, group NeighborsGroup,
	connectOpts ...ConnectPeerOption,
) error {
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
	nbr.Events.Disconnected.Hook(event.NewClosure(func(_ *NeighborDisconnectedEvent) {
		m.deleteNeighbor(nbr)
		m.NeighborsEvents(nbr.Group).NeighborRemoved.Trigger(&NeighborRemovedEvent{nbr})
	}))
	nbr.Events.PacketReceived.Attach(event.NewClosure(func(event *NeighborPacketReceivedEvent) {
		if err := m.handlePacket(event.Packet, nbr); err != nil {
			nbr.log.Debugw("Can't handle packet", "err", err)
		}
	}))
	nbr.readLoop()
	nbr.log.Info("Connection established")
	m.neighborsEvents[group].NeighborAdded.Trigger(&NeighborAddedEvent{nbr})

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

func (m *Manager) processBlockPacket(packetBlk *pb.Packet_Block, nbr *Neighbor) {
	if m.blocksRateLimiter != nil {
		m.blocksRateLimiter.Count(nbr.Peer)
	}
	m.Events.BlockReceived.Trigger(&BlockReceivedEvent{Data: packetBlk.Block.GetData(), Peer: nbr.Peer})
}

func (m *Manager) processBlockRequestPacket(packetBlkReq *pb.Packet_BlockRequest, nbr *Neighbor) {
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
	if err := nbr.ps.writePacket(packet); err != nil {
		nbr.log.Warnw("Failed to send requested block back to the neighbor", "err", err)
		nbr.close()
	}
}
