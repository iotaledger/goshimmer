package eviction

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// State represents the state of the eviction and keeps track of the root blocks.
type State struct {
	Events *Events

	rootBlocks           *memstorage.EpochStorage[models.BlockID, bool]
	latestRootBlock      models.BlockID
	latestRootBlockMutex sync.RWMutex
	storage              *storage.Storage
	lastEvictedEpoch     epoch.Index
	evictionMutex        sync.RWMutex
	triggerMutex         sync.Mutex

	optsRootBlocksEvictionDelay epoch.Index
}

// NewState creates a new eviction State.
func NewState(storageInstance *storage.Storage, opts ...options.Option[State]) (state *State) {
	return options.Apply(&State{
		Events:           NewEvents(),
		rootBlocks:       memstorage.NewEpochStorage[models.BlockID, bool](),
		latestRootBlock:  models.EmptyBlockID,
		storage:          storageInstance,
		lastEvictedEpoch: storageInstance.Settings.LatestCommitment().Index(),
	}, opts, func(s *State) {
		if s.importRootBlocksFromStorage() == 0 {
			s.AddRootBlock(models.EmptyBlockID)
		}
	})
}

// EvictUntil triggers the EpochEvicted event for every evicted epoch and evicts all root blocks until the delayed
// root blocks eviction threshold.
func (s *State) EvictUntil(index epoch.Index) {
	s.triggerMutex.Lock()
	defer s.triggerMutex.Unlock()

	s.evictionMutex.Lock()

	lastEvictedEpoch := s.lastEvictedEpoch
	if index <= lastEvictedEpoch {
		s.evictionMutex.Unlock()
		return
	}

	for currentIndex := lastEvictedEpoch; currentIndex < index; currentIndex++ {
		if delayedIndex := s.delayedBlockEvictionThreshold(currentIndex); delayedIndex >= 0 {
			s.rootBlocks.Evict(delayedIndex)
		}
	}
	s.lastEvictedEpoch = index
	s.evictionMutex.Unlock()

	for currentIndex := lastEvictedEpoch + 1; currentIndex <= index; currentIndex++ {
		s.Events.EpochEvicted.Trigger(currentIndex)
	}
}

// LastEvictedEpoch returns the last evicted epoch.
func (s *State) LastEvictedEpoch() (lastEvictedEpoch epoch.Index) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	return s.lastEvictedEpoch
}

// InEvictedEpoch checks if the Block associated with the given id is too old (in a pruned epoch).
func (s *State) InEvictedEpoch(id models.BlockID) (inEvictedEpoch bool) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	return id.Index() <= s.lastEvictedEpoch
}

// AddRootBlock inserts a solid entry point to the seps map.
func (s *State) AddRootBlock(id models.BlockID) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	if id.Index() <= s.delayedBlockEvictionThreshold(s.lastEvictedEpoch) {
		return
	}

	if s.rootBlocks.Get(id.Index(), true).Set(id, true) {
		if err := s.storage.RootBlocks.Store(id); err != nil {
			panic(errors.Errorf("failed to store root block %s: %w", id, err))
		}
	}

	s.updateLatestRootBlock(id)
}

// RemoveRootBlock removes a solid entry points from the map.
func (s *State) RemoveRootBlock(id models.BlockID) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	if rootBlocks := s.rootBlocks.Get(id.Index()); rootBlocks != nil && rootBlocks.Delete(id) {
		if err := s.storage.RootBlocks.Delete(id); err != nil {
			panic(err)
		}
	}
}

// IsRootBlock returns true if the given block is a root block.
func (s *State) IsRootBlock(id models.BlockID) (has bool) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	epochBlocks := s.rootBlocks.Get(id.Index(), false)

	return epochBlocks != nil && epochBlocks.Has(id)
}

// LatestRootBlock returns the latest root block.
func (s *State) LatestRootBlock() models.BlockID {
	s.latestRootBlockMutex.RLock()
	defer s.latestRootBlockMutex.RUnlock()

	return s.latestRootBlock
}

// importRootBlocksFromStorage imports the root blocks from the storage into the cache.
func (s *State) importRootBlocksFromStorage() (importedBlocks int) {
	for currentEpoch := s.lastEvictedEpoch; currentEpoch >= 0 && currentEpoch > s.delayedBlockEvictionThreshold(s.lastEvictedEpoch); currentEpoch-- {
		s.storage.RootBlocks.Stream(currentEpoch, func(rootBlockID models.BlockID) {
			s.AddRootBlock(rootBlockID)

			importedBlocks++
		})
	}

	return
}

// updateLatestRootBlock updates the latest root block.
func (s *State) updateLatestRootBlock(id models.BlockID) {
	s.latestRootBlockMutex.Lock()
	defer s.latestRootBlockMutex.Unlock()

	if id.Index() >= s.latestRootBlock.Index() {
		s.latestRootBlock = id
	}
}

// delayedBlockEvictionThreshold returns the epoch index that is the threshold for delayed rootblocks eviction.
func (s *State) delayedBlockEvictionThreshold(index epoch.Index) (threshold epoch.Index) {
	return index - s.optsRootBlocksEvictionDelay - 1
}

// WithRootBlocksEvictionDelay sets the time since confirmation threshold.
func WithRootBlocksEvictionDelay(delay epoch.Index) options.Option[State] {
	return func(s *State) {
		s.optsRootBlocksEvictionDelay = delay
	}
}
