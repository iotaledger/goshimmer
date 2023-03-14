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

	if s.heaviestConflict == nil {
		s.heaviestConflict = newConflict
		return
	}

	currentConflict := s.heaviestConflict
	for {
		comparison := newConflict.value.CompareTo(currentConflict.value)
		if comparison == Equal {
			panic("different Conflicts should never have the same weight")
		}

		if comparison == Larger {
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

	currentConflict := s.heaviestConflict
	for currentConflict != nil {
		if err := callback(currentConflict.value); err != nil {
			return err
		}

		currentConflict = currentConflict.lighter
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

// type SortedConflictsInterface[ConflictID, ResourceID IDType] interface {
// 	Add(*Conflict[ConflictID, ResourceID])
//
// 	Remove(ConflictID)
//
// 	HighestPreferredConflict() *Conflict[ConflictID, ResourceID]
//
// 	Size() int
// }
