package warpsync

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/network/warpsync"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/runtime/options"
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

	active  atomic.Bool
	stopped atomic.Bool

	blockLoaderFunc    LoadBlockFunc
	blockProcessorFunc ProcessBlockFunc

	concurrency    int
	blockBatchSize int

	validationInProgress bool
	validationLock       sync.RWMutex

	syncingInProgress bool
	syncingLock       sync.RWMutex
	slotsChannels     map[slot.Index]*slotChannels

	successfulSyncSlot slot.Index

	sync.RWMutex
}

type slotChannels struct {
	sync.RWMutex
	startChan chan *slotSyncStart
	blockChan chan *slotSyncBlock
	endChan   chan *slotSyncEnd
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

// WithConcurrency allows to set how many slots can be requested at once.
func WithConcurrency(concurrency int) options.Option[Manager] {
	return func(m *Manager) {
		m.concurrency = concurrency
	}
}

// WithBlockBatchSize allows to set the size of the block batch returned as part of slot blocks response.
func WithBlockBatchSize(blockBatchSize int) options.Option[Manager] {
	return func(m *Manager) {
		m.blockBatchSize = blockBatchSize
	}
}

/*
func (m *Manager) WarpRange(ctx context.Context, start, end slot.Index, startEC commitment.ID, endPrevEC commitment.ID) (err error) {
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
	if end-m.successfulSyncSlot < minimumWindowSize {
		m.log.Debugf("WarpRange: already synced to %d", m.successfulSyncSlot)
		return nil
	}

	m.active.Set()
	defer m.active.UnSet()

	m.log.Infof("warpsyncing range %d-%d on chain %s -> %s", start, end, startEC.Base58(), endPrevEC.Base58())

	lowestProcessedSlot, syncRangeErr := m.syncRange(ctx, start, end, startEC, ecChain, validPeers)
	if syncRangeErr != nil {
		return errors.Wrapf(syncRangeErr, "failed to sync range %d-%d with peers %s", start, end, validPeers)
	}

	m.log.Infof("range %d-%d synced", start, lowestProcessedSlot)

	m.successfulSyncSlot = lowestProcessedSlot + 1

	return nil
}
*/

// IsStopped returns true if the manager is stopped.
func (m *Manager) IsStopped() bool {
	return m.stopped.Load()
}

// Stop stops the manager and closes all established connections.
func (m *Manager) Stop() {
	m.stopped.Store(false)
	m.protocol.Stop()
}
