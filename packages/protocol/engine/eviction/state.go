package eviction

import (
	"io"
	"math"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/ringbuffer"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/options"
)

// State represents the state of the eviction and keeps track of the root blocks.
type State struct {
	Events *Events

	rootBlocks       *memstorage.SlotStorage[models.BlockID, commitment.ID]
	latestRootBlocks *ringbuffer.RingBuffer[models.BlockID]
	storage          *storage.Storage
	lastEvictedSlot  slot.Index
	evictionMutex    sync.RWMutex
	triggerMutex     sync.Mutex

	optsRootBlocksEvictionDelay slot.Index
}

// NewState creates a new eviction State.
func NewState(storageInstance *storage.Storage, opts ...options.Option[State]) (state *State) {
	return options.Apply(&State{
		Events:                      NewEvents(),
		rootBlocks:                  memstorage.NewSlotStorage[models.BlockID, commitment.ID](),
		latestRootBlocks:            ringbuffer.NewRingBuffer[models.BlockID](8),
		storage:                     storageInstance,
		lastEvictedSlot:             storageInstance.Settings.LatestCommitment().Index(),
		optsRootBlocksEvictionDelay: 3,
	}, opts)
}

// EvictUntil triggers the SlotEvicted event for every evicted slot and evicts all root blocks until the delayed
// root blocks eviction threshold.
func (s *State) EvictUntil(index slot.Index) {
	s.triggerMutex.Lock()
	defer s.triggerMutex.Unlock()

	s.evictionMutex.Lock()

	lastEvictedSlot := s.lastEvictedSlot
	if index <= lastEvictedSlot {
		s.evictionMutex.Unlock()
		return
	}

	for currentIndex := lastEvictedSlot; currentIndex < index; currentIndex++ {
		if delayedIndex := s.delayedBlockEvictionThreshold(currentIndex); delayedIndex >= 0 {
			s.rootBlocks.Evict(delayedIndex)
		}
	}
	s.lastEvictedSlot = index
	s.evictionMutex.Unlock()

	for currentIndex := lastEvictedSlot + 1; currentIndex <= index; currentIndex++ {
		s.Events.SlotEvicted.Trigger(currentIndex)
	}
}

// LastEvictedSlot returns the last evicted slot.
func (s *State) LastEvictedSlot() slot.Index {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	return s.lastEvictedSlot
}

// EarliestRootCommitmentID returns the earliest commitment that rootblocks are committing to across all rootblocks.
func (s *State) EarliestRootCommitmentID() (earliestCommitment commitment.ID) {
	earliestCommitment.SlotIndex = math.MaxInt64
	s.rootBlocks.ForEach(func(index slot.Index, storage *shrinkingmap.ShrinkingMap[models.BlockID, commitment.ID]) {
		storage.ForEach(func(id models.BlockID, commitmentID commitment.ID) bool {
			if commitmentID.Index() < earliestCommitment.Index() {
				earliestCommitment = commitmentID
			}

			return true
		})
	})

	if earliestCommitment.Index() == math.MaxInt64 {
		return commitment.NewEmptyCommitment().ID()
	}

	return earliestCommitment
}

// InEvictedSlot checks if the Block associated with the given id is too old (in a pruned slot).
func (s *State) InEvictedSlot(id models.BlockID) bool {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	return id.Index() <= s.lastEvictedSlot
}

// AddRootBlock inserts a solid entry point to the seps map.
func (s *State) AddRootBlock(id models.BlockID, commitmentID commitment.ID) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	if id.Index() <= s.delayedBlockEvictionThreshold(s.lastEvictedSlot) {
		return
	}

	if s.rootBlocks.Get(id.Index(), true).Set(id, commitmentID) {
		if err := s.storage.RootBlocks.Store(id, commitmentID); err != nil {
			panic(errors.Wrapf(err, "failed to store root block %s", id))
		}
	}

	s.latestRootBlocks.Add(id)
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

	if id.Index() <= s.delayedBlockEvictionThreshold(s.lastEvictedSlot) || id.Index() > s.lastEvictedSlot {
		return false
	}

	slotBlocks := s.rootBlocks.Get(id.Index(), false)

	return slotBlocks != nil && slotBlocks.Has(id)
}

// LatestRootBlocks returns the latest root blocks.
func (s *State) LatestRootBlocks() models.BlockIDs {
	rootBlocks := s.latestRootBlocks.Elements()
	if len(rootBlocks) == 0 {
		return models.NewBlockIDs(models.EmptyBlockID)
	}
	return models.NewBlockIDs(rootBlocks...)
}

// Export exports the root blocks to the given writer.
func (s *State) Export(writer io.WriteSeeker, evictedSlot slot.Index) (err error) {
	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		for currentSlot := s.delayedBlockEvictionThreshold(evictedSlot) + 1; currentSlot <= evictedSlot; currentSlot++ {
			if err = s.storage.RootBlocks.Stream(currentSlot, func(rootBlockID models.BlockID, commitmentID commitment.ID) (err error) {
				if err = stream.WriteSerializable(writer, rootBlockID, models.BlockIDLength); err != nil {
					return errors.Wrapf(err, "failed to write root block ID %s", rootBlockID)
				}

				if err = stream.WriteSerializable(writer, commitmentID, commitmentID.Length()); err != nil {
					return errors.Wrapf(err, "failed to write root block's %s commitment %s", rootBlockID, commitmentID)
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
	var commitmentID commitment.ID

	return stream.ReadCollection(reader, func(i int) error {
		if err = stream.ReadSerializable(reader, &rootBlockID, models.BlockIDLength); err != nil {
			return errors.Wrapf(err, "failed to read root block id %d", i)
		}
		if err = stream.ReadSerializable(reader, &commitmentID, commitmentID.Length()); err != nil {
			return errors.Wrapf(err, "failed to read root block's %s commitment id", rootBlockID)
		}

		s.AddRootBlock(rootBlockID, commitmentID)

		return nil
	})
}

// PopulateFromStorage populates the root blocks from the storage.
func (s *State) PopulateFromStorage(latestCommitmentIndex slot.Index) {
	for index := latestCommitmentIndex - s.delayedBlockEvictionThreshold(latestCommitmentIndex); index <= latestCommitmentIndex; index++ {
		_ = s.storage.RootBlocks.Stream(index, func(id models.BlockID, commitmentID commitment.ID) error {
			s.AddRootBlock(id, commitmentID)

			return nil
		})
	}
}

// delayedBlockEvictionThreshold returns the slot index that is the threshold for delayed rootblocks eviction.
func (s *State) delayedBlockEvictionThreshold(index slot.Index) (threshold slot.Index) {
	return (index - s.optsRootBlocksEvictionDelay - 1).Max(0)
}

// WithRootBlocksEvictionDelay sets the time since confirmation threshold.
func WithRootBlocksEvictionDelay(delay slot.Index) options.Option[State] {
	return func(s *State) {
		s.optsRootBlocksEvictionDelay = delay
	}
}
