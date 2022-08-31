package warpsync

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/typeutils"
)

const (
	protocolID = "warpsync/0.0.1"
)

const minimumWindowSize = 10

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(blockId tangleold.BlockID) (*tangleold.Block, error)

// ProcessBlockFunc defines a function that processes block's bytes from a given peer.
type ProcessBlockFunc func(blk *tangleold.Block, peer *peer.Peer)

// The Manager handles the connected neighbors.
type Manager struct {
	p2pManager *p2p.Manager

	log *logger.Logger

	active  typeutils.AtomicBool
	stopped typeutils.AtomicBool

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
	epochsChannels    map[epoch.Index]*epochChannels

	successfulSyncEpoch epoch.Index

	sync.RWMutex
}

type epochChannels struct {
	sync.RWMutex
	startChan chan *epochSyncStart
	blockChan chan *epochSyncBlock
	endChan   chan *epochSyncEnd
	stopChan  chan struct{}
	active    bool
}

// NewManager creates a new Manager.
func NewManager(p2pManager *p2p.Manager, blockLoaderFunc LoadBlockFunc, blockProcessorFunc ProcessBlockFunc, log *logger.Logger, opts ...options.Option[Manager]) *Manager {
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

	options.Apply(m, opts)

	return m
}

// WithConcurrency allows to set how many epochs can be requested at once.
func WithConcurrency(concurrency int) options.Option[Manager] {
	return func(m *Manager) {
		m.concurrency = concurrency
	}
}

// WithBlockBatchSize allows to set the size of the block batch returned as part of epoch blocks response.
func WithBlockBatchSize(blockBatchSize int) options.Option[Manager] {
	return func(m *Manager) {
		m.blockBatchSize = blockBatchSize
	}
}

func (m *Manager) WarpRange(ctx context.Context, start, end epoch.Index, startEC epoch.EC, endPrevEC epoch.EC) (err error) {
	if m.IsStopped() {
		return errors.Errorf("warpsync manager is stopped")
	}

	if m.active.IsSet() {
		m.log.Debugf("WarpRange: already syncing or validating")
		return nil
	}

	m.Lock()
	defer m.Unlock()

	// Skip warpsyncing if the requested range overlaps with a previous run.
	if end-m.successfulSyncEpoch < minimumWindowSize {
		m.log.Debugf("WarpRange: already synced to %d", m.successfulSyncEpoch)
		return nil
	}

	m.active.Set()
	defer m.active.UnSet()

	m.log.Infof("warpsyncing range %d-%d on chain %s -> %s", start, end, startEC.Base58(), endPrevEC.Base58())

	ecChain, validPeers, validateErr := m.validateBackwards(ctx, start, end, startEC, endPrevEC)
	if validateErr != nil {
		return errors.Wrapf(validateErr, "failed to validate range %d-%d", start, end)
	}
	lowestProcessedEpoch, syncRangeErr := m.syncRange(ctx, start, end, startEC, ecChain, validPeers)
	if syncRangeErr != nil {
		return errors.Wrapf(syncRangeErr, "failed to sync range %d-%d with peers %s", start, end, validPeers)
	}

	m.log.Infof("range %d-%d synced", start, lowestProcessedEpoch)

	m.successfulSyncEpoch = lowestProcessedEpoch + 1

	return nil
}

// IsStopped returns true if the manager is stopped.
func (m *Manager) IsStopped() bool {
	return m.stopped.IsSet()
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.stopped.Set()
	m.p2pManager.UnregisterProtocol(protocolID)
}

func submitTask[P any](packetProcessor func(packet P, nbr *p2p.Neighbor), packet P, nbr *p2p.Neighbor) error {
	if added := event.Loop.TrySubmit(func() { packetProcessor(packet, nbr) }); !added {
		return errors.Errorf("WorkerPool full: packet block discarded")
	}
	return nil
}
