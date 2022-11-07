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

	cache            *memstorage.EpochStorage[models.BlockID, bool]
	storage          *storage.Storage
	lastEvictedEpoch epoch.Index
	evictionMutex    sync.RWMutex

	optsTimeSinceConfirmationThreshold time.Duration
}

func NewState(storageInstance *storage.Storage) *State {
	return &State{
		Events: NewEvents(),

		cache:            memstorage.NewEpochStorage[models.BlockID, bool](),
		storage:          storageInstance,
		lastEvictedEpoch: -1,
	}
}

func (r *State) EvictUntil(index epoch.Index) {
	r.evictionMutex.Lock()
	defer r.evictionMutex.Unlock()

	for currentIndex := r.lastEvictedEpoch + 1; currentIndex <= index; currentIndex++ {
		r.Events.EpochEvicted.Trigger(currentIndex)

		r.cache.Evict(currentIndex)
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

	if id.Index() <= r.lastEvictedEpoch {
		return
	}

	if r.cache.Get(id.Index(), true).Set(id, true) {
		if err := r.storage.RootBlocks.Store(id); err != nil {
			panic(errors.Errorf("failed to store root block %s: %w", id, err))
		}
	}
}

// RemoveRootBlock removes a solid entry points from the map.
func (r *State) RemoveRootBlock(id models.BlockID) {
	r.evictionMutex.Lock()
	defer r.evictionMutex.Unlock()

	if rootBlocks := r.cache.Get(id.Index()); rootBlocks != nil && rootBlocks.Delete(id) {
		if err := r.storage.RootBlocks.Delete(id); err != nil {
			panic(err)
		}
	}
}

func (r *State) IsRootBlock(id models.BlockID) (has bool) {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	epochBlocks := r.cache.Get(id.Index(), false)

	return epochBlocks != nil && epochBlocks.Has(id)
}

func (r *State) LatestRootBlock() models.BlockID {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	// TODO: implement

	return models.EmptyBlockID
}

// WithTimeSinceConfirmationThreshold sets the time since confirmation threshold.
func WithTimeSinceConfirmationThreshold[ID epoch.IndexedID](threshold time.Duration) options.Option[State] {
	return func(e *State) {
		e.optsTimeSinceConfirmationThreshold = threshold
	}
}
