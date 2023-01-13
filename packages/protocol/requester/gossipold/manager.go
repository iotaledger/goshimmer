package gossipold

//
// import (
// 	"fmt"
// 	"sync"
//
// 	"github.com/pkg/errors"
// 	"github.com/iotaledger/hive.go/core/generics/event"
// 	"github.com/iotaledger/hive.go/core/identity"
// 	"github.com/iotaledger/hive.go/core/logger"
// 	"go.uber.org/atomic"
// 	"google.golang.org/protobuf/proto"
//
// 	gp "github.com/iotaledger/goshimmer/packages/network/gossip/gossipproto"
//
// 	"github.com/iotaledger/goshimmer/packages/app/ratelimiter"
// 	"github.com/iotaledger/goshimmer/packages/network"
// )
//
// const (
// 	protocolID = "gossip/0.0.1"
// )
//
// // The Manager handles the connected neighbors.
// type Manager struct {
// 	network network.Network
//
// 	Events *Events
//
// 	log *logger.Logger
//
// 	stopMutex sync.RWMutex
// 	isStopped bool
//
// 	blocksRateLimiter        *ratelimiter.PeerRateLimiter
// 	blockRequestsRateLimiter *ratelimiter.PeerRateLimiter
//
// 	pendingCount          atomic.Uint64
// 	requesterPendingCount atomic.Uint64
// }
//
// // ManagerOption configures the Manager instance.
// type ManagerOption func(m *Manager)
//
// // NewManager creates a new Manager.
// func NewManager(network network.Network, log *logger.Logger, opts ...ManagerOption) *Manager {
// 	m := &Manager{
// 		network: network,
// 		Events:  NewEvents(),
// 		log:     log,
// 	}
//
// 	m.network.RegisterProtocol(protocolID, gossipPacketFactory, m.handlePacket)
//
// 	for _, opt := range opts {
// 		opt(m)
// 	}
//
// 	return m
// }
//
// // WithBlocksRateLimiter allows to set a PeerRateLimiter instance
// // to be used as blocks rate limiter in the gossip manager.
// func WithBlocksRateLimiter(prl *ratelimiter.PeerRateLimiter) ManagerOption {
// 	return func(m *Manager) {
// 		m.blocksRateLimiter = prl
// 	}
// }
//
// // BlocksRateLimiter returns the blocks rate limiter instance used in the gossip manager.
// func (m *Manager) BlocksRateLimiter() *ratelimiter.PeerRateLimiter {
// 	return m.blocksRateLimiter
// }
//
// // WithBlockRequestsRateLimiter allows to set a PeerRateLimiter instance
// // to be used as blocks requests rate limiter in the gossip manager.
// func WithBlockRequestsRateLimiter(prl *ratelimiter.PeerRateLimiter) ManagerOption {
// 	return func(m *Manager) {
// 		m.blockRequestsRateLimiter = prl
// 	}
// }
//
// // BlockRequestsRateLimiter returns the block requests rate limiter instance used in the gossip manager.
// func (m *Manager) BlockRequestsRateLimiter() *ratelimiter.PeerRateLimiter {
// 	return m.blockRequestsRateLimiter
// }
//
// // Stop stops the manager and closes all established connections.
// func (m *Manager) Stop() {
// 	m.stopMutex.Lock()
// 	defer m.stopMutex.Unlock()
//
// 	if m.isStopped {
// 		return
// 	}
// 	m.isStopped = true
// 	m.network.UnregisterProtocol(protocolID)
// }
//
// func (m *Manager) handlePacket(id identity.ID, packet proto.Message) error {
// 	gpPacket := packet.(*gp.Packet)
// 	switch packetBody := gpPacket.GetBody().(type) {
// 	case *gp.Packet_Block:
// 		if added := event.Loop.TrySubmit(func() { m.processBlockPacket(packetBody, id); m.pendingCount.Dec() }); !added {
// 			return errors.New("blockWorkerPool full: packet block discarded")
// 		}
// 		m.pendingCount.Inc()
// 	case *gp.Packet_BlockRequest:
// 		if added := event.Loop.TrySubmit(func() { m.processBlockRequestPacket(packetBody, id); m.requesterPendingCount.Dec() }); !added {
// 			return errors.New("blockRequestWorkerPool full: block request discarded")
// 		}
// 		m.requesterPendingCount.Inc()
// 	default:
// 		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", gpPacket, packetBody, packetBody)
// 	}
//
// 	return nil
// }
//
// // BlockWorkerPoolStatus returns the name and the load of the workerpool.
// func (m *Manager) BlockWorkerPoolStatus() (name string, load uint64) {
// 	return "blockWorkerPool", m.pendingCount.Load()
// }
//
// // BlockRequestWorkerPoolStatus returns the name and the load of the workerpool.
// func (m *Manager) BlockRequestWorkerPoolStatus() (name string, load uint64) {
// 	return "blockRequestWorkerPool", m.requesterPendingCount.Load()
// }
//
// func gossipPacketFactory() proto.Message {
// 	return &gp.Packet{}
// }
