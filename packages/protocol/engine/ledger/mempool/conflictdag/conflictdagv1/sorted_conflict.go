package conflictdagv1

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/acceptance"
	"github.com/iotaledger/goshimmer/packages/core/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

// sortedConflict is a wrapped Conflict that contains additional information for the SortedConflicts.
type sortedConflict[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]] struct {
	// sortedSet is the SortedConflicts that contains this sortedConflict.
	sortedSet *SortedConflicts[ConflictID, ResourceID, VotePower]

	// lighterMember is the sortedConflict that is lighter than this one.
	lighterMember *sortedConflict[ConflictID, ResourceID, VotePower]

	// heavierMember is the sortedConflict that is heavierMember than this one.
	heavierMember *sortedConflict[ConflictID, ResourceID, VotePower]

	// currentWeight is the current weight of the Conflict.
	currentWeight weight.Value

	// queuedWeight is the weight that is queued to be applied to the Conflict.
	queuedWeight *weight.Value

	// weightMutex is used to protect the currentWeight and queuedWeight.
	weightMutex sync.RWMutex

	// currentPreferredInstead is the current PreferredInstead value of the Conflict.
	currentPreferredInstead *Conflict[ConflictID, ResourceID, VotePower]

	// queuedPreferredInstead is the PreferredInstead value that is queued to be applied to the Conflict.
	queuedPreferredInstead *Conflict[ConflictID, ResourceID, VotePower]

	// preferredMutex is used to protect the currentPreferredInstead and queuedPreferredInstead.
	preferredInsteadMutex sync.RWMutex

	onAcceptanceStateUpdatedHook *event.Hook[func(acceptance.State, acceptance.State)]

	// onWeightUpdatedHook is the hook that is triggered when the weight of the Conflict is updated.
	onWeightUpdatedHook *event.Hook[func(weight.Value)]

	// onPreferredUpdatedHook is the hook that is triggered when the PreferredInstead value of the Conflict is updated.
	onPreferredUpdatedHook *event.Hook[func(*Conflict[ConflictID, ResourceID, VotePower])]

	// Conflict is the wrapped Conflict.
	*Conflict[ConflictID, ResourceID, VotePower]
}

// newSortedConflict creates a new sortedConflict.
func newSortedConflict[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](set *SortedConflicts[ConflictID, ResourceID, VotePower], conflict *Conflict[ConflictID, ResourceID, VotePower]) *sortedConflict[ConflictID, ResourceID, VotePower] {
	s := &sortedConflict[ConflictID, ResourceID, VotePower]{
		sortedSet:               set,
		currentWeight:           conflict.Weight.Value(),
		currentPreferredInstead: conflict.PreferredInstead(),
		Conflict:                conflict,
	}

	if set.owner != nil {
		s.onAcceptanceStateUpdatedHook = conflict.AcceptanceStateUpdated.Hook(s.onAcceptanceStateUpdated)
	}

	s.onWeightUpdatedHook = conflict.Weight.OnUpdate.Hook(s.queueWeightUpdate)
	s.onPreferredUpdatedHook = conflict.PreferredInsteadUpdated.Hook(s.queuePreferredInsteadUpdate)

	return s
}

// Weight returns the current weight of the sortedConflict.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) Weight() weight.Value {
	s.weightMutex.RLock()
	defer s.weightMutex.RUnlock()

	return s.currentWeight
}

// Compare compares the sortedConflict to another sortedConflict.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) Compare(other *sortedConflict[ConflictID, ResourceID, VotePower]) int {
	if result := s.Weight().Compare(other.Weight()); result != weight.Equal {
		return result
	}

	return bytes.Compare(lo.PanicOnErr(s.ID.Bytes()), lo.PanicOnErr(other.ID.Bytes()))
}

// PreferredInstead returns the current preferred instead value of the sortedConflict.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) PreferredInstead() *Conflict[ConflictID, ResourceID, VotePower] {
	s.preferredInsteadMutex.RLock()
	defer s.preferredInsteadMutex.RUnlock()

	return s.currentPreferredInstead
}

// IsPreferred returns true if the sortedConflict is preferred instead of its Conflicts.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) IsPreferred() bool {
	return s.PreferredInstead() == s.Conflict
}

// Unhook cleans up the sortedConflict.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) Unhook() {
	if s.onAcceptanceStateUpdatedHook != nil {
		s.onAcceptanceStateUpdatedHook.Unhook()
		s.onAcceptanceStateUpdatedHook = nil
	}

	if s.onWeightUpdatedHook != nil {
		s.onWeightUpdatedHook.Unhook()
		s.onWeightUpdatedHook = nil
	}

	if s.onPreferredUpdatedHook != nil {
		s.onPreferredUpdatedHook.Unhook()
		s.onPreferredUpdatedHook = nil
	}
}

func (s *sortedConflict[ConflictID, ResourceID, VotePower]) onAcceptanceStateUpdated(_, newState acceptance.State) {
	if newState.IsAccepted() {
		s.sortedSet.owner.setAcceptanceState(acceptance.Rejected)
	}
}

// queueWeightUpdate queues a weight update for the sortedConflict.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) queueWeightUpdate(newWeight weight.Value) {
	s.weightMutex.Lock()
	defer s.weightMutex.Unlock()

	if (s.queuedWeight == nil && s.currentWeight == newWeight) || (s.queuedWeight != nil && *s.queuedWeight == newWeight) {
		return
	}

	s.queuedWeight = &newWeight
	s.sortedSet.notifyPendingWeightUpdate(s)
}

// weightUpdateApplied tries to apply a queued weight update to the sortedConflict and returns true if successful.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) weightUpdateApplied() bool {
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
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) queuePreferredInsteadUpdate(conflict *Conflict[ConflictID, ResourceID, VotePower]) {
	s.preferredInsteadMutex.Lock()
	defer s.preferredInsteadMutex.Unlock()

	if (s.queuedPreferredInstead == nil && s.currentPreferredInstead == conflict) ||
		(s.queuedPreferredInstead != nil && s.queuedPreferredInstead == conflict) ||
		s.sortedSet.owner.Conflict == conflict {
		return
	}

	s.queuedPreferredInstead = conflict
	s.sortedSet.notifyPendingPreferredInsteadUpdate(s)
}

// preferredInsteadUpdateApplied tries to apply a queued preferred instead update to the sortedConflict and returns
// true if successful.
func (s *sortedConflict[ConflictID, ResourceID, VotePower]) preferredInsteadUpdateApplied() bool {
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
