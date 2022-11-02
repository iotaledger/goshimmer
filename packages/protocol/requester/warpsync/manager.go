package warpsync

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/typeutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/network/warpsync"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const minimumWindowSize = 10

// LoadBlockFunc defines a function that returns the block for the given id.
type LoadBlockFunc func(models.BlockID) (*models.Block, bool)

// ProcessBlockFunc defines a function that processes block's bytes from a given peer.
type ProcessBlockFunc func(*p2p.Neighbor, *models.Block)

// The Manager handles the connected neighbors.
type Manager struct {
	protocol          *warpsync.Protocol
	commitmentManager *chainmanager.Manager

	log *logger.Logger

	active  typeutils.AtomicBool
	stopped typeutils.AtomicBool

	blockLoaderFunc    LoadBlockFunc
	blockProcessorFunc ProcessBlockFunc

	concurrency    int
	blockBatchSize int

	validationInProgress bool
	validationLock       sync.RWMutex

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
func NewManager(blockLoaderFunc LoadBlockFunc, blockProcessorFunc ProcessBlockFunc, log *logger.Logger, opts ...options.Option[Manager]) *Manager {
	m := &Manager{
		log:                log,
		blockLoaderFunc:    blockLoaderFunc,
		blockProcessorFunc: blockProcessorFunc,
	}

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

/*
func (m *Manager) WarpRange(ctx context.Context, start, end epoch.Index, startEC commitment.ID, endPrevEC commitment.ID) (err error) {
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

	lowestProcessedEpoch, syncRangeErr := m.syncRange(ctx, start, end, startEC, ecChain, validPeers)
	if syncRangeErr != nil {
		return errors.Wrapf(syncRangeErr, "failed to sync range %d-%d with peers %s", start, end, validPeers)
	}

	m.log.Infof("range %d-%d synced", start, lowestProcessedEpoch)

	m.successfulSyncEpoch = lowestProcessedEpoch + 1

	return nil
}
*/

// IsStopped returns true if the manager is stopped.
func (m *Manager) IsStopped() bool {
	return m.stopped.IsSet()
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.stopped.Set()
	m.protocol.Stop()
}
