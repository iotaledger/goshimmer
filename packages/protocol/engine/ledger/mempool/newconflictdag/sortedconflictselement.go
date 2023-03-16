package newconflictdag

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

type SortedConflictsElement[ConflictID, ResourceID IDType] struct {
	container     *SortedConflicts[ConflictID, ResourceID]
	conflict      *Conflict[ConflictID, ResourceID]
	currentWeight weight.Value
	queuedWeight  *weight.Value
	lighter       *SortedConflictsElement[ConflictID, ResourceID]
	heavier       *SortedConflictsElement[ConflictID, ResourceID]
	onUpdateHook  *event.Hook[func(weight.Value)]
	mutex         sync.RWMutex
}

func newSortedConflict[ConflictID, ResourceID IDType](container *SortedConflicts[ConflictID, ResourceID], conflict *Conflict[ConflictID, ResourceID]) *SortedConflictsElement[ConflictID, ResourceID] {
	s := new(SortedConflictsElement[ConflictID, ResourceID])
	s.container = container
	s.conflict = conflict
	s.onUpdateHook = conflict.Weight().OnUpdate.Hook(s.setQueuedWeight)
	s.currentWeight = conflict.Weight().Value()

	return s
}

func (s *SortedConflictsElement[ConflictID, ResourceID]) CurrentWeight() weight.Value {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.currentWeight
}

func (s *SortedConflictsElement[ConflictID, ResourceID]) Compare(other *SortedConflictsElement[ConflictID, ResourceID]) int {
	if result := s.CurrentWeight().Compare(other.CurrentWeight()); result != weight.Equal {
		return result
	}

	return bytes.Compare(lo.PanicOnErr(s.conflict.id.Bytes()), lo.PanicOnErr(other.conflict.id.Bytes()))
}

func (s *SortedConflictsElement[ConflictID, ResourceID]) setQueuedWeight(newWeight weight.Value) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.queuedWeight = &newWeight

	s.container.queueUpdate(s)
}

func (s *SortedConflictsElement[ConflictID, ResourceID]) updateWeight() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.queuedWeight == nil {
		return false
	}

	s.currentWeight = *s.queuedWeight
	s.queuedWeight = nil

	return true
}
