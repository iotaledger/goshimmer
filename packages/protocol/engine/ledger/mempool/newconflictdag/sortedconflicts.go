package newconflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/stringify"
)

type SortedConflicts[ConflictID, ResourceID IDType] struct {
	conflictsByID    *shrinkingmap.ShrinkingMap[ConflictID, *sortedConflict[ConflictID, ResourceID]]
	heaviestConflict *sortedConflict[ConflictID, ResourceID]

	mutex sync.RWMutex
}

func NewSortedConflicts[ConflictID, ResourceID IDType]() *SortedConflicts[ConflictID, ResourceID] {
	return &SortedConflicts[ConflictID, ResourceID]{
		conflictsByID: shrinkingmap.New[ConflictID, *sortedConflict[ConflictID, ResourceID]](),
	}
}

func (s *SortedConflicts[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newConflict := &sortedConflict[ConflictID, ResourceID]{value: conflict}
	if !s.conflictsByID.Set(conflict.id, newConflict) {
		return
	}

	conflict.OnWeightUpdated.Hook(s.onConflictWeightUpdated)

	if s.heaviestConflict == nil {
		s.heaviestConflict = newConflict
		return
	}

	for currentConflict := s.heaviestConflict; ; {
		comparison := newConflict.value.CompareTo(currentConflict.value)
		if comparison == Equal {
			panic("different Conflicts should never have the same weight")
		}

		if comparison == Larger {
			if currentConflict.heavier != nil {
				currentConflict.heavier.lighter = newConflict
			}

			newConflict.lighter = currentConflict
			newConflict.heavier = currentConflict.heavier
			currentConflict.heavier = newConflict

			if currentConflict == s.heaviestConflict {
				s.heaviestConflict = newConflict
			}

			break
		}

		if currentConflict.lighter == nil {
			currentConflict.lighter = newConflict
			newConflict.heavier = currentConflict
			break
		}

		currentConflict = currentConflict.lighter
	}
}

func (s *SortedConflicts[ConflictID, ResourceID]) ForEach(callback func(*Conflict[ConflictID, ResourceID]) error) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for currentConflict := s.heaviestConflict; currentConflict != nil; currentConflict = currentConflict.lighter {
		if err := callback(currentConflict.value); err != nil {
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

func (s *SortedConflicts[ConflictID, ResourceID]) onConflictWeightUpdated(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	updatedConflict, exists := s.conflictsByID.Get(conflict.id)
	if !exists || updatedConflict.value != conflict {
		panic("the Conflict that was updated was not found in the SortedConflicts")
	}

	for currentConflict := updatedConflict.heavier; currentConflict != nil && currentConflict.value.CompareTo(updatedConflict.value) == Smaller; currentConflict = updatedConflict.heavier {
		s.swapConflicts(updatedConflict, currentConflict)
	}

	for lighterConflict := updatedConflict.lighter; lighterConflict != nil && lighterConflict.value.CompareTo(updatedConflict.value) == Larger; lighterConflict = updatedConflict.lighter {
		s.swapConflicts(lighterConflict, updatedConflict)
	}
}

// swapConflicts swaps the two given Conflicts in the SortedConflicts while assuming that the heavierConflict is heavier than the lighterConflict.
func (s *SortedConflicts[ConflictID, ResourceID]) swapConflicts(heavierConflict *sortedConflict[ConflictID, ResourceID], lighterConflict *sortedConflict[ConflictID, ResourceID]) {
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

type sortedConflict[ConflictID, ResourceID IDType] struct {
	value   *Conflict[ConflictID, ResourceID]
	lighter *sortedConflict[ConflictID, ResourceID]
	heavier *sortedConflict[ConflictID, ResourceID]
}
