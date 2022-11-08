package eviction

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type State struct {
	Events *Events

	rootBlocks       *memstorage.EpochStorage[models.BlockID, bool]
	storage          *storage.Storage
	lastEvictedEpoch epoch.Index
	evictionMutex    sync.RWMutex
	triggerMutex     sync.Mutex

	optsRootBlockEvictionDelay         epoch.Index
	optsTimeSinceConfirmationThreshold time.Duration
}

func NewState(storageInstance *storage.Storage, opts ...options.Option[State]) (state *State) {
	return options.Apply(&State{
		Events:           NewEvents(),
		rootBlocks:       memstorage.NewEpochStorage[models.BlockID, bool](),
		storage:          storageInstance,
		lastEvictedEpoch: storageInstance.Settings.LatestCommitment().Index(),
	}, opts, func(s *State) {
		if s.importRootBlocksFromStorage() == 0 {
			s.rootBlocks.Get(0, true).Set(models.EmptyBlockID, true)
		}
	})
}

func (r *State) importRootBlocksFromStorage() (importedBlocks int) {
	for currentEpoch := r.lastEvictedEpoch; currentEpoch >= 0 && currentEpoch > r.delayedBlockEvictionThreshold(r.lastEvictedEpoch); currentEpoch-- {
		r.storage.RootBlocks.Stream(currentEpoch, func(rootBlockID models.BlockID) {
			r.rootBlocks.Get(rootBlockID.Index(), true).Set(rootBlockID, true)

			importedBlocks++
		})
	}

	return
}

func (r *State) EvictUntil(index epoch.Index) {
	r.evictionMutex.Lock()
	r.triggerMutex.Lock()
	defer r.triggerMutex.Unlock()

	lastEvictedEpoch := r.lastEvictedEpoch
	if index <= lastEvictedEpoch {
		r.evictionMutex.Unlock()
		return
	}

	for currentIndex := lastEvictedEpoch; currentIndex < index; currentIndex++ {
		if delayedIndex := r.delayedBlockEvictionThreshold(currentIndex); delayedIndex >= 0 {
			r.rootBlocks.Evict(delayedIndex)
		}
	}
	r.lastEvictedEpoch = index
	r.evictionMutex.Unlock()

	for currentIndex := lastEvictedEpoch + 1; currentIndex <= index; currentIndex++ {
		r.Events.EpochEvicted.Trigger(currentIndex)
	}
}

func (r *State) LastEvictedEpoch() (lastEvictedEpoch epoch.Index) {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	return r.lastEvictedEpoch
}

// InEvictedEpoch checks if the Block associated with the given id is too old (in a pruned epoch).
func (r *State) InEvictedEpoch(id models.BlockID) (inEvictedEpoch bool) {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	return id.Index() <= r.lastEvictedEpoch
}

// AddRootBlock inserts a solid entry point to the seps map.
func (r *State) AddRootBlock(id models.BlockID) {
	r.evictionMutex.Lock()
	defer r.evictionMutex.Unlock()

	if id.Index() <= r.delayedBlockEvictionThreshold(r.lastEvictedEpoch) {
		return
	}

	if r.rootBlocks.Get(id.Index(), true).Set(id, true) {
		if err := r.storage.RootBlocks.Store(id); err != nil {
			panic(errors.Errorf("failed to store root block %s: %w", id, err))
		}
	}
}

func (r *State) delayedBlockEvictionThreshold(index epoch.Index) (threshold epoch.Index) {
	return index - r.optsRootBlockEvictionDelay - 1
}

// RemoveRootBlock removes a solid entry points from the map.
func (r *State) RemoveRootBlock(id models.BlockID) {
	r.evictionMutex.Lock()
	defer r.evictionMutex.Unlock()

	if rootBlocks := r.rootBlocks.Get(id.Index()); rootBlocks != nil && rootBlocks.Delete(id) {
		if err := r.storage.RootBlocks.Delete(id); err != nil {
			panic(err)
		}
	}
}

func (r *State) IsRootBlock(id models.BlockID) (has bool) {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	epochBlocks := r.rootBlocks.Get(id.Index(), false)

	return epochBlocks != nil && epochBlocks.Has(id)
}

func (r *State) LatestRootBlock() models.BlockID {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	// TODO: implement

	return models.EmptyBlockID
}

// WithRootBlocksEvictionDelay sets the time since confirmation threshold.
func WithRootBlocksEvictionDelay(delay epoch.Index) options.Option[State] {
	return func(e *State) {
		e.optsRootBlockEvictionDelay = delay
	}
}
