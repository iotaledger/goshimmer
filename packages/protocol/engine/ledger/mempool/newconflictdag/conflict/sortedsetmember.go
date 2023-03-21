package conflict

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/reentrantmutex"
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
	onPreferredUpdatedHook *event.Hook[func(*Conflict[ConflictID, ResourceID], reentrantmutex.ThreadID)]

	// Conflict is the wrapped Conflict.
	*Conflict[ConflictID, ResourceID]
}

// newSortedSetMember creates a new sortedSetMember.
func newSortedSetMember[ConflictID, ResourceID IDType](set *SortedSet[ConflictID, ResourceID], conflict *Conflict[ConflictID, ResourceID]) *sortedSetMember[ConflictID, ResourceID] {
	s := &sortedSetMember[ConflictID, ResourceID]{
		sortedSet:     set,
		currentWeight: conflict.Weight().Value(),
		Conflict:      conflict,
	}

	s.onUpdateHook = conflict.Weight().OnUpdate.Hook(s.queueWeightUpdate)

	// do not attach to event from ourselves
	if set.owner != conflict {
		s.onPreferredUpdatedHook = conflict.PreferredInsteadUpdated.Hook(func(newPreferredConflict *Conflict[ConflictID, ResourceID], threadID reentrantmutex.ThreadID) {
			s.notifyPreferredInsteadUpdate(newPreferredConflict, threadID)
		})
	}

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

	return bytes.Compare(lo.PanicOnErr(s.id.Bytes()), lo.PanicOnErr(other.id.Bytes()))
}

// Dispose cleans up the sortedSetMember.
func (s *sortedSetMember[ConflictID, ResourceID]) Dispose() {
	s.onUpdateHook.Unhook()
	s.onPreferredUpdatedHook.Unhook()
}

// queueWeightUpdate queues a weight update for the sortedSetMember.
func (s *sortedSetMember[ConflictID, ResourceID]) queueWeightUpdate(newWeight weight.Value) {
	s.weightMutex.Lock()
	defer s.weightMutex.Unlock()

	if (s.queuedWeight == nil && s.currentWeight == newWeight) || (s.queuedWeight != nil && *s.queuedWeight == newWeight) {
		return
	}

	s.queuedWeight = &newWeight
	s.sortedSet.notifyPendingWeightUpdate(s)
}

// weightUpdateApplied tries to apply a queued weight update to the sortedSetMember and returns true if successful.
func (s *sortedSetMember[ConflictID, ResourceID]) weightUpdateApplied() bool {
	s.weightMutex.Lock()
	defer s.weightMutex.Unlock()

	if s.queuedWeight == nil {
		return false
	}

	s.currentWeight = *s.queuedWeight
	s.queuedWeight = nil

	return true
}

// notifyPreferredInsteadUpdate notifies the sortedSet that the preferred instead flag of the Conflict was updated.
func (s *sortedSetMember[ConflictID, ResourceID]) notifyPreferredInsteadUpdate(newPreferredConflict *Conflict[ConflictID, ResourceID], threadID reentrantmutex.ThreadID) {
	s.sortedSet.notifyPreferredInsteadUpdate(s, newPreferredConflict == s.Conflict, threadID)
}
