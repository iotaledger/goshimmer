package warpsync

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
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

	log *logger.Logger

	stopMutex sync.RWMutex
	isStopped bool

	blockLoaderFunc    LoadBlockFunc
	blockProcessorFunc ProcessBlockFunc

	concurrency    int
	blockBatchSize int

	validationInProgress bool
	validationLock       sync.RWMutex
	commitmentsChan      chan *neighborCommitment
	commitmentsStopChan  chan struct{}

	syncingInProgress bool
	syncingLock       sync.RWMutex
	epochChannels     map[epoch.Index]*epochChannels

	sync.RWMutex
}

type epochChannels struct {
	sync.RWMutex
	startChan chan *epochSyncStart
	blockChan chan *epochSyncBlock
	endChan   chan *epochSyncEnd
	stopChan  chan struct{}
}

// ManagerOption configures the Manager instance.
type ManagerOption func(*Manager)

// NewManager creates a new Manager.
func NewManager(p2pManager *p2p.Manager, blockLoaderFunc LoadBlockFunc, blockProcessorFunc ProcessBlockFunc, log *logger.Logger, opts ...ManagerOption) *Manager {
	m := &Manager{
		p2pManager:         p2pManager,
		log:                log,
		blockLoaderFunc:    blockLoaderFunc,
		blockProcessorFunc: blockProcessorFunc,
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

func (m *Manager) WarpRange(ctx context.Context, start, end epoch.Index, startEC epoch.EC, endPrevEC epoch.EC) (err error) {
	m.Lock()
	defer m.Unlock()

	ecChain, validPeers, validateErr := m.validateBackwards(ctx, start, end, startEC, endPrevEC)
	if validateErr != nil {
		return errors.Wrapf(validateErr, "failed to validate range %d-%d with peers %s", start, end)
	}
	if syncRangeErr := m.syncRange(ctx, start, end, startEC, ecChain, validPeers); syncRangeErr != nil {
		return errors.Wrapf(syncRangeErr, "failed to sync range %d-%d with peers %s", start, end, validPeers)
	}

	return nil
}

// IsStopped returns true if the manager is stopped.
func (m *Manager) IsStopped() bool {
	m.stopMutex.RLock()
	defer m.stopMutex.RUnlock()

	return m.isStopped
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

func submitTask[P any](packetProcessor func(packet P, nbr *p2p.Neighbor), packet P, nbr *p2p.Neighbor) error {
	if added := event.Loop.TrySubmit(func() { packetProcessor(packet, nbr) }); !added {
		return errors.Errorf("WorkerPool full: packet block discarded")
	}
	return nil
}
