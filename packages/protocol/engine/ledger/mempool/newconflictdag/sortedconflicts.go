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
	conflictsByID    *shrinkingmap.ShrinkingMap[ConflictID, *SortedConflictsElement[ConflictID, ResourceID]]
	heaviestConflict *SortedConflictsElement[ConflictID, ResourceID]

	updates       map[ConflictID]*SortedConflictsElement[ConflictID, ResourceID]
	updatesMutex  sync.RWMutex
	updatesSignal *sync.Cond

	isShutdown atomic.Bool

	pendingUpdatesCounter *syncutils.Counter

	mutex sync.RWMutex
}

func NewSortedConflicts[ConflictID, ResourceID IDType]() *SortedConflicts[ConflictID, ResourceID] {
	s := &SortedConflicts[ConflictID, ResourceID]{
		conflictsByID:         shrinkingmap.New[ConflictID, *SortedConflictsElement[ConflictID, ResourceID]](),
		updates:               make(map[ConflictID]*SortedConflictsElement[ConflictID, ResourceID]),
		pendingUpdatesCounter: syncutils.NewCounter(),
	}
	s.updatesSignal = sync.NewCond(&s.updatesMutex)

	go s.updateWorker()

	return s
}

func (s *SortedConflicts[ConflictID, ResourceID]) Wait() {
	s.pendingUpdatesCounter.WaitIsZero()
}

func (s *SortedConflicts[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newSortedConflict := newSortedConflict[ConflictID, ResourceID](s, conflict)
	if !s.conflictsByID.Set(conflict.id, newSortedConflict) {
		return
	}

	go func() {}()

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

func (s *SortedConflicts[ConflictID, ResourceID]) String() string {
	structBuilder := stringify.NewStructBuilder("SortedConflicts")

	_ = s.ForEach(func(conflict *Conflict[ConflictID, ResourceID]) error {
		structBuilder.AddField(stringify.NewStructField(conflict.id.String(), conflict))

		return nil
	})

	return structBuilder.String()
}

func (s *SortedConflicts[ConflictID, ResourceID]) updateWorker() {
	for conflict := s.nextUpdate(); conflict != nil; conflict = s.nextUpdate() {
		if conflict.applyQueuedWeight() {
			s.fixOrderOf(conflict.conflict.id)
		}
	}
}

func (s *SortedConflicts[ConflictID, ResourceID]) fixOrderOf(conflictID ConflictID) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	updatedConflict, exists := s.conflictsByID.Get(conflictID)
	if !exists {
		panic("the Conflict that was updated was not found in the SortedConflicts")
	}

	for currentConflict := updatedConflict.heavier; currentConflict != nil && currentConflict.Compare(updatedConflict) == weight.Lighter; currentConflict = updatedConflict.heavier {
		s.swapConflicts(updatedConflict, currentConflict)
	}

	for lighterConflict := updatedConflict.lighter; lighterConflict != nil && lighterConflict.Compare(updatedConflict) == weight.Heavier; lighterConflict = updatedConflict.lighter {
		s.swapConflicts(lighterConflict, updatedConflict)
	}
}

// swapConflicts swaps the two given Conflicts in the SortedConflicts while assuming that the heavierConflict is heavier than the lighterConflict.
func (s *SortedConflicts[ConflictID, ResourceID]) swapConflicts(heavierConflict *SortedConflictsElement[ConflictID, ResourceID], lighterConflict *SortedConflictsElement[ConflictID, ResourceID]) {
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

func (s *SortedConflicts[ConflictID, ResourceID]) notifyUpdate(conflict *SortedConflictsElement[ConflictID, ResourceID]) {
	s.updatesMutex.Lock()
	defer s.updatesMutex.Unlock()

	if _, exists := s.updates[conflict.conflict.id]; exists {
		return
	}

	s.pendingUpdatesCounter.Increase()

	s.updates[conflict.conflict.id] = conflict

	s.updatesSignal.Signal()
}

// nextUpdate returns the next SortedConflictsElement that needs to be updated (or nil if the shutdown flag is set).
func (s *SortedConflicts[ConflictID, ResourceID]) nextUpdate() *SortedConflictsElement[ConflictID, ResourceID] {
	s.updatesMutex.Lock()
	defer s.updatesMutex.Unlock()

	for !s.isShutdown.Load() && len(s.updates) == 0 {
		s.updatesSignal.Wait()
	}

	if !s.isShutdown.Load() {
		for conflictID, conflict := range s.updates {
			delete(s.updates, conflictID)

			s.pendingUpdatesCounter.Decrease()

			return conflict
		}
	}

	return nil
}
