package warpsync

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"google.golang.org/protobuf/proto"
)

const (
	protocolID = "warpsync/0.0.1"
)

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(blockId tangle.BlockID) (*tangle.Block, error)

// ProcessBlockFunc defines a function that processes block's bytes from a given peer.
type ProcessBlockFunc func(blockBytes []byte, peer *peer.Peer)

// The Manager handles the connected neighbors.
type Manager struct {
	p2pManager *p2p.Manager

	Events *Events

	log *logger.Logger

	stopMutex sync.RWMutex
	isStopped bool

	blockLoaderFunc    LoadBlockFunc
	blockProcessorFunc ProcessBlockFunc

	concurrency    int
	blockBatchSize int

	validationInProgress bool
	commitmentsChan      chan (*epoch.ECRecord)

	syncingInProgress       bool
	epochSyncStartChan      chan (*epochSyncStart)
	epochSyncBatchBlockChan chan (*epochSyncBatchBlock)
	epochSyncEndChan        chan (*epochSyncEnd)

	sync.RWMutex
}

// ManagerOption configures the Manager instance.
type ManagerOption func(*Manager)

// NewManager creates a new Manager.
func NewManager(p2pManager *p2p.Manager, blockLoaderFunc LoadBlockFunc, blockProcessorFunc ProcessBlockFunc, log *logger.Logger, opts ...ManagerOption) *Manager {
	m := &Manager{
		p2pManager:              p2pManager,
		Events:                  newEvents(),
		log:                     log,
		blockLoaderFunc:         blockLoaderFunc,
		blockProcessorFunc:      blockProcessorFunc,
		commitmentsChan:         make(chan *epoch.ECRecord),
		epochSyncStartChan:      make(chan *epochSyncStart),
		epochSyncBatchBlockChan: make(chan *epochSyncBatchBlock),
		epochSyncEndChan:        make(chan *epochSyncEnd),
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

// WithBlockBatchSize allows to set the size of the block batch returned as part of epoch blocks response.
func WithBlockBatchSize(blockBatchSize int) ManagerOption {
	return func(m *Manager) {
		m.blockBatchSize = blockBatchSize
	}
}

// IsStopped returns true if the manager is stopped.
func (m *Manager) IsStopped() bool {
	m.RLock()
	defer m.RUnlock()

	return m.isStopped
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.stopMutex.Lock()
	defer m.stopMutex.Unlock()

	if m.isStopped {
		return
	}

	close(m.commitmentsChan)
	close(m.epochSyncStartChan)
	close(m.epochSyncBatchBlockChan)
	close(m.epochSyncEndChan)

	m.isStopped = true
	m.p2pManager.UnregisterProtocol(protocolID)
}

func (m *Manager) handlePacket(nbr *p2p.Neighbor, packet proto.Message) error {
	wpPacket := packet.(*wp.Packet)
	switch packetBody := wpPacket.GetBody().(type) {
	case *wp.Packet_EpochBlocksRequest:
		return submitTask(m.processEpochBlocksRequestPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksStart:
		return submitTask(m.processEpochBlocksStartPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksBatch:
		return submitTask(m.processEpochBlocksBatchPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksEnd:
		return submitTask(m.processEpochBlocksEndPacket, packetBody, nbr)
	case *wp.Packet_EpochCommitmentRequest:
		return submitTask(m.processEpochCommittmentRequestPacket, packetBody, nbr)
	case *wp.Packet_EpochCommitment:
		return submitTask(m.processEpochCommittmentPacket, packetBody, nbr)
	default:
		return errors.Newf("unsupported packet; packet=%+v, packetBody=%T-%+v", wpPacket, packetBody, packetBody)
	}
}

func submitTask[P any](packetProcessor func(packet P, nbr *p2p.Neighbor), packet P, nbr *p2p.Neighbor) error {
	if added := event.Loop.TrySubmit(func() { packetProcessor(packet, nbr) }); !added {
		return errors.Errorf("WorkerPool full: packet block discarded")
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
