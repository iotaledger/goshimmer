package conflict

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

// sortedSetMember is a wrapped Conflict that contains additional information for the SortedSet.
type sortedSetMember[ConflictID, ResourceID IDType] struct {
	// sortedSet is the SortedSet that contains this sortedSetMember.
	sortedSet *SortedSet[ConflictID, ResourceID]

	// lighterMember is the sortedSetMember that is lighter than this one.
	lighterMember *sortedSetMember[ConflictID, ResourceID]

	// heavierMember is the sortedSetMember that is heavierMember than this one.
	heavierMember *sortedSetMember[ConflictID, ResourceID]

	// currentWeight is the current weight of the Conflict.
	currentWeight weight.Value

	// queuedWeight is the weight that is queued to be applied to the Conflict.
	queuedWeight *weight.Value

	// weightMutex is used to protect the currentWeight and queuedWeight.
	weightMutex sync.RWMutex

	// onUpdateHook is the hook that is triggered when the weight of the Conflict is updated.
	onUpdateHook *event.Hook[func(weight.Value)]

	// onPreferredUpdatedHook is the hook that is triggered when the preferredInstead value of the Conflict is updated.
	onPreferredUpdatedHook *event.Hook[func(*Conflict[ConflictID, ResourceID])]

	// Conflict is the wrapped Conflict.
	Conflict *Conflict[ConflictID, ResourceID]
}

// newSortedConflictElement creates a new sortedSetMember.
func newSortedConflictElement[ConflictID, ResourceID IDType](
	sortedSet *SortedSet[ConflictID, ResourceID],
	wrappedConflict *Conflict[ConflictID, ResourceID],
) *sortedSetMember[ConflictID, ResourceID] {
	s := &sortedSetMember[ConflictID, ResourceID]{
		sortedSet:     sortedSet,
		currentWeight: wrappedConflict.Weight().Value(),
		Conflict:      wrappedConflict,
	}

	s.onUpdateHook = wrappedConflict.Weight().OnUpdate.Hook(s.setQueuedWeight)
	s.onPreferredUpdatedHook = wrappedConflict.PreferredInsteadUpdated.Hook(s.onPreferredUpdated)

	return s
}

// Weight returns the current weight of the sortedSetMember.
func (s *sortedSetMember[ConflictID, ResourceID]) Weight() weight.Value {
	s.weightMutex.RLock()
	defer s.weightMutex.RUnlock()

	return s.currentWeight
}

// Compare compares the sortedSetMember to another sortedSetMember.
func (s *sortedSetMember[ConflictID, ResourceID]) Compare(other *sortedSetMember[ConflictID, ResourceID]) int {
	if result := s.Weight().Compare(other.Weight()); result != weight.Equal {
		return result
	}

	return bytes.Compare(lo.PanicOnErr(s.Conflict.id.Bytes()), lo.PanicOnErr(other.Conflict.id.Bytes()))
}

func (s *sortedSetMember[ConflictID, ResourceID]) setQueuedWeight(newWeight weight.Value) {
	s.weightMutex.Lock()
	defer s.weightMutex.Unlock()

	if (s.queuedWeight == nil && s.currentWeight == newWeight) || (s.queuedWeight != nil && *s.queuedWeight == newWeight) {
		return
	}

	s.queuedWeight = &newWeight

	s.sortedSet.queueUpdate(s)
}

func (s *sortedSetMember[ConflictID, ResourceID]) updateWeight() bool {
	s.weightMutex.Lock()
	defer s.weightMutex.Unlock()

	if s.queuedWeight == nil {
		return false
	}

	s.currentWeight = *s.queuedWeight
	s.queuedWeight = nil

	return true
}

func (s *sortedSetMember[ConflictID, ResourceID]) onPreferredUpdated(preferredInstead *Conflict[ConflictID, ResourceID]) {
	if preferredInstead == s.Conflict {
		s.sortedSet.markConflictPreferred(s)
	} else {
		s.sortedSet.markConflictNotPreferred(s)
	}
}
