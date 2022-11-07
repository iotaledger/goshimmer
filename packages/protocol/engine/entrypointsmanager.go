package engine

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type RootBlocksManager struct {
	cache   *memstorage.Storage[epoch.Index, *set.AdvancedSet[models.BlockID]]
	storage *storage.Storage

	optsTimeSinceConfirmationThreshold time.Duration

	sync.RWMutex
}

func NewRootBlocksManager(s *storage.Storage) *RootBlocksManager {
	return &RootBlocksManager{
		cache:   memstorage.New[epoch.Index, *set.AdvancedSet[models.BlockID]](),
		storage: s,
	}
}

// Evict remove seps of old epoch when confirmed epoch advanced.
func (e *RootBlocksManager) Evict(ei epoch.Index) {
	e.Lock()
	defer e.Unlock()

	e.cache.Delete(ei)
}

// Insert inserts a solid entry point to the seps map.
func (e *RootBlocksManager) Insert(id models.BlockID) {
	e.Lock()
	defer e.Unlock()

	if lo.Return1(e.cache.RetrieveOrCreate(id.Index(), func() *set.AdvancedSet[models.BlockID] {
		return set.NewAdvancedSet[models.BlockID]()
	})).Add(id) {
		e.storage.RootBlocks.Store(id)
	}
}

// Remove removes a solid entry points from the map.
func (e *RootBlocksManager) Remove(id models.BlockID) {
	e.Lock()
	defer e.Unlock()

	if rootBlocksForEpoch, exists := e.cache.Get(id.Index()); !exists || !rootBlocksForEpoch.Delete(id) {
		return
	}

	e.storage.RootBlocks.Delete(id)
}

func (e *RootBlocksManager) IsRootBlock(id models.BlockID) (has bool) {
	e.RLock()
	defer e.RUnlock()

	if rootBlocksForEpoch, exists := e.cache.Get(id.Index()); exists {
		return rootBlocksForEpoch.Has(id)
	}

	return false
}

func (e *RootBlocksManager) LatestRootBlockID() models.BlockID {
	e.RLock()
	defer e.RUnlock()

	return models.EmptyBlockID
}

// WithTimeSinceConfirmationThreshold sets the time since confirmation threshold.
func WithTimeSinceConfirmationThreshold[ID epoch.IndexedID](threshold time.Duration) options.Option[RootBlocksManager] {
	return func(e *RootBlocksManager) {
		e.optsTimeSinceConfirmationThreshold = threshold
	}
}
