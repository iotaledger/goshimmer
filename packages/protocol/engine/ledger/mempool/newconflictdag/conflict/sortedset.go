package conflict

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

type SortedSet[ConflictID, ResourceID IDType] struct {
	HeaviestPreferredConflictUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

	owner                   *Conflict[ConflictID, ResourceID]
	conflicts               *shrinkingmap.ShrinkingMap[ConflictID, *sortedSetMember[ConflictID, ResourceID]]
	heaviestConflict        *sortedSetMember[ConflictID, ResourceID]
	heaviestPreferredMember *sortedSetMember[ConflictID, ResourceID]

	pendingUpdates        map[ConflictID]*sortedSetMember[ConflictID, ResourceID]
	pendingUpdatesCounter *syncutils.Counter
	pendingUpdatesSignal  *sync.Cond
	pendingUpdatesMutex   sync.RWMutex

	isShutdown atomic.Bool

	mutex sync.RWMutex
}

func NewSortedConflicts[ConflictID, ResourceID IDType](owner *Conflict[ConflictID, ResourceID]) *SortedSet[ConflictID, ResourceID] {
	s := &SortedSet[ConflictID, ResourceID]{
		HeaviestPreferredConflictUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		owner:                            owner,
		conflicts:                        shrinkingmap.New[ConflictID, *sortedSetMember[ConflictID, ResourceID]](),
		pendingUpdates:                   make(map[ConflictID]*sortedSetMember[ConflictID, ResourceID]),
		pendingUpdatesCounter:            syncutils.NewCounter(),
	}
	s.pendingUpdatesSignal = sync.NewCond(&s.pendingUpdatesMutex)

	s.Add(owner)

	go s.updateWorker()

	return s
}

func (s *SortedSet[ConflictID, ResourceID]) HeaviestPreferredConflict() *Conflict[ConflictID, ResourceID] {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.heaviestPreferredMember == nil {
		return nil
	}

	return s.heaviestPreferredMember.Conflict
}

func (s *SortedSet[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newSortedConflict, isNew := s.conflicts.GetOrCreate(conflict.id, func() *sortedSetMember[ConflictID, ResourceID] {
		return newSortedSetMember[ConflictID, ResourceID](s, conflict)
	})

	if !isNew {
		return
	}

	if conflict == s.owner || (conflict.IsPreferred() && newSortedConflict.Compare(s.heaviestPreferredMember) == weight.Heavier) {
		s.heaviestPreferredMember = newSortedConflict

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
			if currentConflict.heavierMember != nil {
				currentConflict.heavierMember.lighterMember = newSortedConflict
			}

			newSortedConflict.lighterMember = currentConflict
			newSortedConflict.heavierMember = currentConflict.heavierMember
			currentConflict.heavierMember = newSortedConflict

			if currentConflict == s.heaviestConflict {
				s.heaviestConflict = newSortedConflict
			}

			break
		}

		if currentConflict.lighterMember == nil {
			currentConflict.lighterMember = newSortedConflict
			newSortedConflict.heavierMember = currentConflict
			break
		}

		currentConflict = currentConflict.lighterMember
	}
}

func (s *SortedSet[ConflictID, ResourceID]) ForEach(callback func(*Conflict[ConflictID, ResourceID]) error) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for currentConflict := s.heaviestConflict; currentConflict != nil; currentConflict = currentConflict.lighterMember {
		if err := callback(currentConflict.Conflict); err != nil {
			return err
		}
	}

	return nil
}

func (s *SortedSet[ConflictID, ResourceID]) WaitSorted() {
	s.pendingUpdatesCounter.WaitIsZero()
}

func (s *SortedSet[ConflictID, ResourceID]) String() string {
	structBuilder := stringify.NewStructBuilder("SortedConflicts")
	_ = s.ForEach(func(conflict *Conflict[ConflictID, ResourceID]) error {
		structBuilder.AddField(stringify.NewStructField(conflict.id.String(), conflict))
		return nil
	})

	return structBuilder.String()
}

func (s *SortedSet[ConflictID, ResourceID]) queueUpdate(conflict *sortedSetMember[ConflictID, ResourceID]) {
	s.pendingUpdatesMutex.Lock()
	defer s.pendingUpdatesMutex.Unlock()

	if _, exists := s.pendingUpdates[conflict.id]; !exists {
		s.pendingUpdatesCounter.Increase()
		s.pendingUpdates[conflict.id] = conflict
		s.pendingUpdatesSignal.Signal()
	}
}

// nextConflictToUpdate returns the next sortedSetMember that needs to be updated (or nil if the shutdown flag is set).
func (s *SortedSet[ConflictID, ResourceID]) nextConflictToUpdate() *sortedSetMember[ConflictID, ResourceID] {
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

func (s *SortedSet[ConflictID, ResourceID]) updateWorker() {
	for conflict := s.nextConflictToUpdate(); conflict != nil; conflict = s.nextConflictToUpdate() {
		if conflict.updateWeight() {
			s.fixPosition(conflict)
		}
	}
}

func (s *SortedSet[ConflictID, ResourceID]) fixPosition(updatedConflict *sortedSetMember[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// the updatedConflict needs to be moved up in the list
	updatedConflictIsPreferred := (updatedConflict.Conflict == s.owner && updatedConflict == s.heaviestPreferredMember) || updatedConflict.IsPreferred()
	for currentConflict := updatedConflict.heavierMember; currentConflict != nil && currentConflict.Compare(updatedConflict) == weight.Lighter; currentConflict = updatedConflict.heavierMember {
		s.swapNeighbors(updatedConflict, currentConflict)

		if updatedConflictIsPreferred && currentConflict == s.heaviestPreferredMember {
			s.heaviestPreferredMember = updatedConflict
			s.HeaviestPreferredConflictUpdated.Trigger(updatedConflict.Conflict)
		}
	}

	// the updatedConflict needs to be moved down in the list
	updatedConflictIsHeaviestPreferredConflict := updatedConflict == s.heaviestPreferredMember
	for lighterConflict := updatedConflict.lighterMember; lighterConflict != nil && lighterConflict.Compare(updatedConflict) == weight.Heavier; lighterConflict = updatedConflict.lighterMember {
		s.swapNeighbors(lighterConflict, updatedConflict)

		if updatedConflictIsHeaviestPreferredConflict && lighterConflict.IsPreferred() {
			s.heaviestPreferredMember = lighterConflict
			s.HeaviestPreferredConflictUpdated.Trigger(lighterConflict.Conflict)

			updatedConflictIsHeaviestPreferredConflict = false
		}
	}
}

// swapNeighbors swaps the two given Conflicts in the SortedConflicts while assuming that the heavierConflict is heavier than the lighterConflict.
func (s *SortedSet[ConflictID, ResourceID]) swapNeighbors(heavierConflict *sortedSetMember[ConflictID, ResourceID], lighterConflict *sortedSetMember[ConflictID, ResourceID]) {
	if heavierConflict.lighterMember != nil {
		heavierConflict.lighterMember.heavierMember = lighterConflict
	}
	if lighterConflict.heavierMember != nil {
		lighterConflict.heavierMember.lighterMember = heavierConflict
	}

	lighterConflict.lighterMember = heavierConflict.lighterMember
	heavierConflict.heavierMember = lighterConflict.heavierMember
	lighterConflict.heavierMember = heavierConflict
	heavierConflict.lighterMember = lighterConflict

	if s.heaviestConflict == lighterConflict {
		s.heaviestConflict = heavierConflict
	}
}

func (s *SortedSet[ConflictID, ResourceID]) markConflictNotPreferred(member *sortedSetMember[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.heaviestPreferredMember != member {
		return
	}

	currentConflict := member.lighterMember
	for currentConflict.Conflict != s.owner && !currentConflict.IsPreferred() && currentConflict.PreferredInstead() != member.Conflict {
		currentConflict = currentConflict.lighterMember
	}

	s.heaviestPreferredMember = currentConflict
	s.HeaviestPreferredConflictUpdated.Trigger(s.heaviestPreferredMember.Conflict)
}

func (s *SortedSet[ConflictID, ResourceID]) markConflictPreferred(member *sortedSetMember[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if member.Compare(s.heaviestPreferredMember) == weight.Heavier {
		s.heaviestPreferredMember = member
		s.HeaviestPreferredConflictUpdated.Trigger(member.Conflict)
	}
}
