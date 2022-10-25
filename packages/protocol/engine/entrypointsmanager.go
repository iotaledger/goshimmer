package engine

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type EntryPointsManager struct {
	entryPoints *shrinkingmap.ShrinkingMap[epoch.Index, models.BlockIDs]

	sync.RWMutex
}

func NewEntryPointsManager() *EntryPointsManager {
	return &EntryPointsManager{
		entryPoints: shrinkingmap.New[epoch.Index, models.BlockIDs](),
	}
}

func (e *EntryPointsManager) SolidEntryPoints(index epoch.Index) (entryPoints *set.AdvancedSet[models.BlockID]) {
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

// EvictSolidEntryPoints remove seps of old epoch when confirmed epoch advanced.
func (e *EntryPointsManager) EvictSolidEntryPoints(ei epoch.Index) {
	e.Lock()
	defer e.Unlock()

	// preserve seps of last confirmed epoch until now
	e.entryPoints.Delete(ei)
}

// InsertSolidEntryPoint inserts a solid entry point to the seps map.
func (e *EntryPointsManager) InsertSolidEntryPoint(id models.BlockID) {
	e.Lock()
	defer e.Unlock()

	sep, ok := e.entryPoints.Get(id.EpochIndex)
	if !ok {
		sep = make(map[models.BlockID]types.Empty)
	}

	sep[id] = types.Void
	e.entryPoints.Set(id.EpochIndex, sep)
}

// RemoveSolidEntryPoint removes a solid entry points from the map.
func (e *EntryPointsManager) RemoveSolidEntryPoint(b *models.Block) (err error) {
	e.Lock()
	defer e.Unlock()

	epochSeps, exists := e.entryPoints.Get(b.ID().EpochIndex)
	if !exists {
		return errors.New("solid entry point of the epoch does not exist")
	}

	delete(epochSeps, b.ID())

	return
}
