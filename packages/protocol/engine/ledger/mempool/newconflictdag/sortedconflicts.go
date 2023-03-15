package newconflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/stringify"
)

type SortedConflicts[ConflictID, ResourceID IDType] struct {
	conflictsByID    *shrinkingmap.ShrinkingMap[ConflictID, *SortedConflict[ConflictID, ResourceID]]
	heaviestConflict *SortedConflict[ConflictID, ResourceID]

	mutex sync.RWMutex
}

func NewSortedConflicts[ConflictID, ResourceID IDType]() *SortedConflicts[ConflictID, ResourceID] {
	return &SortedConflicts[ConflictID, ResourceID]{
		conflictsByID: shrinkingmap.New[ConflictID, *SortedConflict[ConflictID, ResourceID]](),
	}
}

func (s *SortedConflicts[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newConflict := NewSortedConflict(conflict)
	if !s.conflictsByID.Set(conflict.id, newConflict) {
		return
	}

	conflict.OnWeightUpdated.Hook(s.onWeightUpdated)

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

func (s *SortedConflicts[ConflictID, ResourceID]) onWeightUpdated(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sortedConflict, exists := s.conflictsByID.Get(conflict.id)
	if !exists || sortedConflict.value != conflict {
		panic("the Conflict that was updated was not found in the SortedConflicts")
	}

	for currentConflict := sortedConflict.lighter; currentConflict != nil && currentConflict.value.CompareTo(sortedConflict.value) == Larger; currentConflict = currentConflict.lighter {
		currentConflict.heavier = sortedConflict.heavier
		if currentConflict.heavier != nil {
			currentConflict.heavier.lighter = currentConflict
		}

		sortedConflict.lighter = currentConflict.lighter
		if sortedConflict.lighter != nil {
			sortedConflict.lighter.heavier = sortedConflict
		}

		currentConflict.lighter = sortedConflict
		sortedConflict.heavier = currentConflict
	}

	for currentConflict := sortedConflict.heavier; currentConflict != nil && currentConflict.value.CompareTo(sortedConflict.value) == Smaller; currentConflict = currentConflict.heavier {
		currentConflict.lighter = sortedConflict.lighter
		if currentConflict.lighter != nil {
			currentConflict.lighter.heavier = currentConflict
		}

		sortedConflict.heavier = currentConflict.heavier
		if sortedConflict.heavier != nil {
			sortedConflict.heavier.lighter = sortedConflict
		}

		currentConflict.heavier = sortedConflict
		sortedConflict.lighter = currentConflict
	}
}

type SortedConflict[ConflictID, ResourceID IDType] struct {
	value   *Conflict[ConflictID, ResourceID]
	lighter *SortedConflict[ConflictID, ResourceID]
	heavier *SortedConflict[ConflictID, ResourceID]
}

func NewSortedConflict[ConflictID, ResourceID IDType](conflict *Conflict[ConflictID, ResourceID]) *SortedConflict[ConflictID, ResourceID] {
	return &SortedConflict[ConflictID, ResourceID]{
		value: conflict,
	}
}
