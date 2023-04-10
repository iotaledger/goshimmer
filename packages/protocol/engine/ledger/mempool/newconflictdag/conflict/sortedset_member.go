package conflict

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
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

	// currentPreferredInstead is the current PreferredInstead value of the Conflict.
	currentPreferredInstead *Conflict[ConflictID, ResourceID]

	// queuedPreferredInstead is the PreferredInstead value that is queued to be applied to the Conflict.
	queuedPreferredInstead *Conflict[ConflictID, ResourceID]

	// preferredMutex is used to protect the currentPreferredInstead and queuedPreferredInstead.
	preferredInsteadMutex sync.RWMutex

	onAcceptanceStateUpdatedHook *event.Hook[func(acceptance.State, acceptance.State)]

	// onWeightUpdatedHook is the hook that is triggered when the weight of the Conflict is updated.
	onWeightUpdatedHook *event.Hook[func(weight.Value)]

	// onPreferredUpdatedHook is the hook that is triggered when the PreferredInstead value of the Conflict is updated.
	onPreferredUpdatedHook *event.Hook[func(*Conflict[ConflictID, ResourceID])]

	// Conflict is the wrapped Conflict.
	*Conflict[ConflictID, ResourceID]
}

// newSortedSetMember creates a new sortedSetMember.
func newSortedSetMember[ConflictID, ResourceID IDType](set *SortedSet[ConflictID, ResourceID], conflict *Conflict[ConflictID, ResourceID]) *sortedSetMember[ConflictID, ResourceID] {
	s := &sortedSetMember[ConflictID, ResourceID]{
		sortedSet:               set,
		currentWeight:           conflict.Weight.Value(),
		currentPreferredInstead: conflict.PreferredInstead(),
		Conflict:                conflict,
	}

	if conflict != set.owner {
		s.onAcceptanceStateUpdatedHook = conflict.AcceptanceStateUpdated.Hook(s.onAcceptanceStateUpdated)
	}

	s.onWeightUpdatedHook = conflict.Weight.OnUpdate.Hook(s.queueWeightUpdate)
	s.onPreferredUpdatedHook = conflict.PreferredInsteadUpdated.Hook(s.queuePreferredInsteadUpdate)

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

	return bytes.Compare(lo.PanicOnErr(s.ID.Bytes()), lo.PanicOnErr(other.ID.Bytes()))
}

// PreferredInstead returns the current preferred instead value of the sortedSetMember.
func (s *sortedSetMember[ConflictID, ResourceID]) PreferredInstead() *Conflict[ConflictID, ResourceID] {
	s.preferredInsteadMutex.RLock()
	defer s.preferredInsteadMutex.RUnlock()

	return s.currentPreferredInstead
}

// IsPreferred returns true if the sortedSetMember is preferred instead of its Conflicts.
func (s *sortedSetMember[ConflictID, ResourceID]) IsPreferred() bool {
	return s.PreferredInstead() == s.Conflict
}

// Dispose cleans up the sortedSetMember.
func (s *sortedSetMember[ConflictID, ResourceID]) Dispose() {
	if s.onAcceptanceStateUpdatedHook != nil {
		s.onAcceptanceStateUpdatedHook.Unhook()
	}

	s.onWeightUpdatedHook.Unhook()
	s.onPreferredUpdatedHook.Unhook()
}

func (s *sortedSetMember[ConflictID, ResourceID]) onAcceptanceStateUpdated(_, newState acceptance.State) {
	if newState.IsAccepted() {
		s.sortedSet.owner.SetAcceptanceState(acceptance.Rejected)
	}
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

	if *s.queuedWeight == s.currentWeight {
		s.queuedWeight = nil

		return false
	}

	s.currentWeight = *s.queuedWeight
	s.queuedWeight = nil

	return true
}

// queuePreferredInsteadUpdate notifies the sortedSet that the preferred instead flag of the Conflict was updated.
func (s *sortedSetMember[ConflictID, ResourceID]) queuePreferredInsteadUpdate(conflict *Conflict[ConflictID, ResourceID]) {
	s.preferredInsteadMutex.Lock()
	defer s.preferredInsteadMutex.Unlock()

	if (s.queuedPreferredInstead == nil && s.currentPreferredInstead == conflict) || (s.queuedPreferredInstead != nil && s.queuedPreferredInstead == conflict) || s.sortedSet.owner == conflict {
		return
	}

	s.queuedPreferredInstead = conflict
	s.sortedSet.notifyPendingPreferredInsteadUpdate(s)
}

// preferredInsteadUpdateApplied tries to apply a queued preferred instead update to the sortedSetMember and returns
// true if successful.
func (s *sortedSetMember[ConflictID, ResourceID]) preferredInsteadUpdateApplied() bool {
	s.preferredInsteadMutex.Lock()
	defer s.preferredInsteadMutex.Unlock()

	if s.queuedPreferredInstead == nil {
		return false
	}

	if s.queuedPreferredInstead == s.currentPreferredInstead {
		s.queuedPreferredInstead = nil

		return false
	}

	s.currentPreferredInstead = s.queuedPreferredInstead
	s.queuedPreferredInstead = nil

	return true
}
