package eviction

import (
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/runtime/options"
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
		Events:                      NewEvents(),
		rootBlocks:                  memstorage.NewEpochStorage[models.BlockID, bool](),
		latestRootBlock:             models.EmptyBlockID,
		storage:                     storageInstance,
		lastEvictedEpoch:            storageInstance.Settings.LatestCommitment().Index(),
		optsRootBlocksEvictionDelay: 3,
	}, opts)
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
			panic(errors.Wrapf(err, "failed to store root block %s", id))
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
	if id == models.EmptyBlockID {
		return true
	}

	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	if id.Index() <= s.delayedBlockEvictionThreshold(s.lastEvictedEpoch) {
		return false
	}

	epochBlocks := s.rootBlocks.Get(id.Index(), false)

	return epochBlocks != nil && epochBlocks.Has(id)
}

// LatestRootBlock returns the latest root block.
func (s *State) LatestRootBlock() models.BlockID {
	s.latestRootBlockMutex.RLock()
	defer s.latestRootBlockMutex.RUnlock()

	return s.latestRootBlock
}

// Export exports the root blocks to the given writer.
func (s *State) Export(writer io.WriteSeeker, evictedEpoch epoch.Index) (err error) {
	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		for currentEpoch := s.delayedBlockEvictionThreshold(evictedEpoch) + 1; currentEpoch <= evictedEpoch; currentEpoch++ {
			if err = s.storage.RootBlocks.Stream(currentEpoch, func(rootBlockID models.BlockID) (err error) {
				if err = stream.WriteSerializable(writer, rootBlockID, models.BlockIDLength); err != nil {
					return errors.Wrapf(err, "failed to write root block %s", rootBlockID)
				}

				elementsCount++

				return
			}); err != nil {
				return 0, errors.Wrap(err, "failed to stream root blocks")
			}
		}

		return elementsCount, nil
	})
}

// Import imports the root blocks from the given reader.
func (s *State) Import(reader io.ReadSeeker) (err error) {
	var rootBlockID models.BlockID

	return stream.ReadCollection(reader, func(i int) error {
		if err = stream.ReadSerializable(reader, &rootBlockID, models.BlockIDLength); err != nil {
			return errors.Wrapf(err, "failed to read root block id %d", i)
		}

		s.AddRootBlock(rootBlockID)

		return nil
	})
}

// importRootBlocksFromStorage imports the root blocks from the storage into the cache.
func (s *State) importRootBlocksFromStorage() (importedBlocks int) {
	for currentEpoch := s.lastEvictedEpoch; currentEpoch >= 0 && currentEpoch > s.delayedBlockEvictionThreshold(s.lastEvictedEpoch); currentEpoch-- {
		if err := s.storage.RootBlocks.Stream(currentEpoch, func(rootBlockID models.BlockID) (err error) {
			s.AddRootBlock(rootBlockID)

			importedBlocks++

			return
		}); err != nil {
			panic(errors.Wrap(err, "failed importing root blocks from storage"))
		}
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
	return (index - s.optsRootBlocksEvictionDelay - 1).Max(0)
}

// WithRootBlocksEvictionDelay sets the time since confirmation threshold.
func WithRootBlocksEvictionDelay(delay epoch.Index) options.Option[State] {
	return func(s *State) {
		s.optsRootBlocksEvictionDelay = delay
	}
}
