package newconflictdag

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

type SortedConflicts[ConflictID, ResourceID IDType] struct {
	conflicts        *shrinkingmap.ShrinkingMap[ConflictID, *SortedConflictsElement[ConflictID, ResourceID]]
	heaviestConflict *SortedConflictsElement[ConflictID, ResourceID]

	pendingUpdates        map[ConflictID]*SortedConflictsElement[ConflictID, ResourceID]
	pendingUpdatesCounter *syncutils.Counter
	pendingUpdatesSignal  *sync.Cond
	pendingUpdatesMutex   sync.RWMutex

	isShutdown atomic.Bool

	mutex sync.RWMutex
}

func NewSortedConflicts[ConflictID, ResourceID IDType]() *SortedConflicts[ConflictID, ResourceID] {
	s := &SortedConflicts[ConflictID, ResourceID]{
		conflicts:             shrinkingmap.New[ConflictID, *SortedConflictsElement[ConflictID, ResourceID]](),
		pendingUpdates:        make(map[ConflictID]*SortedConflictsElement[ConflictID, ResourceID]),
		pendingUpdatesCounter: syncutils.NewCounter(),
	}
	s.pendingUpdatesSignal = sync.NewCond(&s.pendingUpdatesMutex)

	go s.updateWorker()

	return s
}

func (s *SortedConflicts[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newSortedConflict := newSortedConflict[ConflictID, ResourceID](s, conflict)
	if !s.conflicts.Set(conflict.id, newSortedConflict) {
		return
	}

	if s.heaviestConflict == nil {
		s.heaviestConflict = newSortedConflict
		return
	}

	for currentConflict := s.heaviestConflict; ; {
		comparison := newSortedConflict.Compare(currentConflict)
		if comparison == weight.Equal {
			panic("different Conflicts should never have the same weight")
		}

		if comparison == weight.Heavier {
			if currentConflict.heavier != nil {
				currentConflict.heavier.lighter = newSortedConflict
			}

			newSortedConflict.lighter = currentConflict
			newSortedConflict.heavier = currentConflict.heavier
			currentConflict.heavier = newSortedConflict

			if currentConflict == s.heaviestConflict {
				s.heaviestConflict = newSortedConflict
			}

			break
		}

		if currentConflict.lighter == nil {
			currentConflict.lighter = newSortedConflict
			newSortedConflict.heavier = currentConflict
			break
		}

		currentConflict = currentConflict.lighter
	}
}

func (s *SortedConflicts[ConflictID, ResourceID]) ForEach(callback func(*Conflict[ConflictID, ResourceID]) error) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for currentConflict := s.heaviestConflict; currentConflict != nil; currentConflict = currentConflict.lighter {
		if err := callback(currentConflict.conflict); err != nil {
			return err
		}
	}

	return nil
}

func (s *SortedConflicts[ConflictID, ResourceID]) WaitSorted() {
	s.pendingUpdatesCounter.WaitIsZero()
}

func (s *SortedConflicts[ConflictID, ResourceID]) String() string {
	structBuilder := stringify.NewStructBuilder("SortedConflicts")
	_ = s.ForEach(func(conflict *Conflict[ConflictID, ResourceID]) error {
		structBuilder.AddField(stringify.NewStructField(conflict.id.String(), conflict))
		return nil
	})

	return structBuilder.String()
}

func (s *SortedConflicts[ConflictID, ResourceID]) queueUpdate(conflict *SortedConflictsElement[ConflictID, ResourceID]) {
	s.pendingUpdatesMutex.Lock()
	defer s.pendingUpdatesMutex.Unlock()

	if _, exists := s.pendingUpdates[conflict.conflict.id]; !exists {
		s.pendingUpdatesCounter.Increase()
		s.pendingUpdates[conflict.conflict.id] = conflict
		s.pendingUpdatesSignal.Signal()
	}
}

// nextConflictToUpdate returns the next SortedConflictsElement that needs to be updated (or nil if the shutdown flag is set).
func (s *SortedConflicts[ConflictID, ResourceID]) nextConflictToUpdate() *SortedConflictsElement[ConflictID, ResourceID] {
	s.pendingUpdatesMutex.Lock()
	defer s.pendingUpdatesMutex.Unlock()

	for !s.isShutdown.Load() && len(s.pendingUpdates) == 0 {
		s.pendingUpdatesSignal.Wait()
	}

	if !s.isShutdown.Load() {
		for conflictID, conflict := range s.pendingUpdates {
			delete(s.pendingUpdates, conflictID)

			s.pendingUpdatesCounter.Decrease()

			return conflict
		}
	}

	return nil
}

func (s *SortedConflicts[ConflictID, ResourceID]) updateWorker() {
	for conflict := s.nextConflictToUpdate(); conflict != nil; conflict = s.nextConflictToUpdate() {
		if conflict.updateWeight() {
			s.fixPosition(conflict.conflict.id)
		}
	}
}

func (s *SortedConflicts[ConflictID, ResourceID]) fixPosition(conflictID ConflictID) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	updatedConflict, exists := s.conflicts.Get(conflictID)
	if !exists {
		panic("the Conflict that was updated was not found in the SortedConflicts")
	}

	for currentConflict := updatedConflict.heavier; currentConflict != nil && currentConflict.Compare(updatedConflict) == weight.Lighter; currentConflict = updatedConflict.heavier {
		s.swapNeighbors(updatedConflict, currentConflict)
	}

	for lighterConflict := updatedConflict.lighter; lighterConflict != nil && lighterConflict.Compare(updatedConflict) == weight.Heavier; lighterConflict = updatedConflict.lighter {
		s.swapNeighbors(lighterConflict, updatedConflict)
	}
}

// swapNeighbors swaps the two given Conflicts in the SortedConflicts while assuming that the heavierConflict is heavier than the lighterConflict.
func (s *SortedConflicts[ConflictID, ResourceID]) swapNeighbors(heavierConflict *SortedConflictsElement[ConflictID, ResourceID], lighterConflict *SortedConflictsElement[ConflictID, ResourceID]) {
	if heavierConflict.lighter != nil {
		heavierConflict.lighter.heavier = lighterConflict
	}
	if lighterConflict.heavier != nil {
		lighterConflict.heavier.lighter = heavierConflict
	}

	lighterConflict.lighter = heavierConflict.lighter
	heavierConflict.heavier = lighterConflict.heavier
	lighterConflict.heavier = heavierConflict
	heavierConflict.lighter = lighterConflict

	if s.heaviestConflict == lighterConflict {
		s.heaviestConflict = heavierConflict
	}
}
