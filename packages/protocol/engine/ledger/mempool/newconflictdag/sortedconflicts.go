package newconflictdag

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

type SortedConflicts[ConflictID, ResourceID IDType] struct {
	HeaviestPreferredConflictUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

	owner                     *Conflict[ConflictID, ResourceID]
	conflicts                 *shrinkingmap.ShrinkingMap[ConflictID, *SortedConflictsElement[ConflictID, ResourceID]]
	heaviestConflict          *SortedConflictsElement[ConflictID, ResourceID]
	heaviestPreferredConflict *SortedConflictsElement[ConflictID, ResourceID]

	pendingUpdates        map[ConflictID]*SortedConflictsElement[ConflictID, ResourceID]
	pendingUpdatesCounter *syncutils.Counter
	pendingUpdatesSignal  *sync.Cond
	pendingUpdatesMutex   sync.RWMutex

	isShutdown atomic.Bool

	mutex sync.RWMutex
}

func NewSortedConflicts[ConflictID, ResourceID IDType](owner *Conflict[ConflictID, ResourceID]) *SortedConflicts[ConflictID, ResourceID] {
	s := &SortedConflicts[ConflictID, ResourceID]{
		HeaviestPreferredConflictUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		owner:                            owner,
		conflicts:                        shrinkingmap.New[ConflictID, *SortedConflictsElement[ConflictID, ResourceID]](),
		pendingUpdates:                   make(map[ConflictID]*SortedConflictsElement[ConflictID, ResourceID]),
		pendingUpdatesCounter:            syncutils.NewCounter(),
	}
	s.pendingUpdatesSignal = sync.NewCond(&s.pendingUpdatesMutex)

	s.Add(owner)

	go s.updateWorker()

	return s
}

func (s *SortedConflicts[ConflictID, ResourceID]) HeaviestPreferredConflict() *Conflict[ConflictID, ResourceID] {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.heaviestPreferredConflict == nil {
		return nil
	}

	return s.heaviestPreferredConflict.conflict
}

func (s *SortedConflicts[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newSortedConflict, isNew := s.conflicts.GetOrCreate(conflict.id, func() *SortedConflictsElement[ConflictID, ResourceID] {
		return newSortedConflictElement[ConflictID, ResourceID](s, conflict)
	})

	if !isNew {
		return
	}

	if conflict == s.owner || (conflict.IsPreferred() && newSortedConflict.Compare(s.heaviestPreferredConflict) == weight.Heavier) {
		s.heaviestPreferredConflict = newSortedConflict

		s.HeaviestPreferredConflictUpdated.Trigger(conflict)
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
			s.fixPosition(conflict)
		}
	}
}

func (s *SortedConflicts[ConflictID, ResourceID]) fixPosition(updatedConflict *SortedConflictsElement[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// the updatedConflict needs to be moved up in the list
	updatedConflictIsPreferred := (updatedConflict.conflict == s.owner && updatedConflict == s.heaviestPreferredConflict) || updatedConflict.conflict.IsPreferred()
	for currentConflict := updatedConflict.heavier; currentConflict != nil && currentConflict.Compare(updatedConflict) == weight.Lighter; currentConflict = updatedConflict.heavier {
		s.swapNeighbors(updatedConflict, currentConflict)

		if updatedConflictIsPreferred && currentConflict == s.heaviestPreferredConflict {
			s.heaviestPreferredConflict = updatedConflict
			s.HeaviestPreferredConflictUpdated.Trigger(updatedConflict.conflict)
		}
	}

	// the updatedConflict needs to be moved down in the list
	updatedConflictIsHeaviestPreferredConflict := updatedConflict == s.heaviestPreferredConflict
	for lighterConflict := updatedConflict.lighter; lighterConflict != nil && lighterConflict.Compare(updatedConflict) == weight.Heavier; lighterConflict = updatedConflict.lighter {
		s.swapNeighbors(lighterConflict, updatedConflict)

		if updatedConflictIsHeaviestPreferredConflict && lighterConflict.conflict.IsPreferred() {
			s.heaviestPreferredConflict = lighterConflict
			s.HeaviestPreferredConflictUpdated.Trigger(lighterConflict.conflict)

			updatedConflictIsHeaviestPreferredConflict = false
		}
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

func (s *SortedConflicts[ConflictID, ResourceID]) markConflictNotPreferred(conflict *SortedConflictsElement[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.heaviestPreferredConflict != conflict {
		return
	}

	currentConflict := conflict.lighter
	for currentConflict.conflict != s.owner && !currentConflict.conflict.IsPreferred() && currentConflict.conflict.PreferredInstead() != conflict.conflict {
		currentConflict = currentConflict.lighter
	}

	s.heaviestPreferredConflict = currentConflict
	s.HeaviestPreferredConflictUpdated.Trigger(s.heaviestPreferredConflict.conflict)
}

func (s *SortedConflicts[ConflictID, ResourceID]) markConflictPreferred(conflict *SortedConflictsElement[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if conflict.Compare(s.heaviestPreferredConflict) == weight.Heavier {
		s.heaviestPreferredConflict = conflict
		s.HeaviestPreferredConflictUpdated.Trigger(conflict.conflict)
	}
}
