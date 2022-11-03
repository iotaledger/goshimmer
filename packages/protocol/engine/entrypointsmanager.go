package engine

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type EntryPointsManager struct {
	entryPoints *shrinkingmap.ShrinkingMap[epoch.Index, models.BlockIDs]
	storage     *storage.Storage

	sync.RWMutex
}

func NewEntryPointsManager(s *storage.Storage) *EntryPointsManager {
	return &EntryPointsManager{
		entryPoints: shrinkingmap.New[epoch.Index, models.BlockIDs](),
		storage:     s,
	}
}

func (e *EntryPointsManager) LoadAll(index epoch.Index) (entryPoints *set.AdvancedSet[models.BlockID]) {
	e.RLock()
	defer e.RUnlock()

	entryPoints = set.NewAdvancedSet[models.BlockID]()
	if entryPointsMap, ok := e.entryPoints.Get(index); ok {
		for entryPoint := range entryPointsMap {
			entryPoints.Add(entryPoint)
		}
	}

	return
}

// Evict remove seps of old epoch when confirmed epoch advanced.
func (e *EntryPointsManager) Evict(ei epoch.Index) {
	e.Lock()
	defer e.Unlock()

	// preserve seps of last confirmed epoch until now
	e.entryPoints.Delete(ei)
}

// Insert inserts a solid entry point to the seps map.
func (e *EntryPointsManager) Insert(id models.BlockID) {
	e.Lock()
	defer e.Unlock()

	sep, ok := e.entryPoints.Get(id.EpochIndex)
	if !ok {
		sep = make(map[models.BlockID]types.Empty)
	}

	sep[id] = types.Void
	e.entryPoints.Set(id.EpochIndex, sep)
	e.storage.EntryPoints.Store(id)
}

// Remove removes a solid entry points from the map.
func (e *EntryPointsManager) Remove(id models.BlockID) (err error) {
	e.Lock()
	defer e.Unlock()

	epochSeps, exists := e.entryPoints.Get(id.Index())
	if !exists {
		return errors.New("solid entry point of the epoch does not exist")
	}

	delete(epochSeps, id)
	e.storage.EntryPoints.Delete(id)

	return
}
