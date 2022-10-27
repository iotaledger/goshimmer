package latestvotes

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/thresholdmap"

	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// LatestVotes keeps track of the most up-to-date for a certain Voter casted on a specific Marker SequenceID.
// Votes can be casted on Markers (SequenceID, Index), but can arrive in any arbitrary order.
// Due to the nature of a Sequence, a vote casted for a certain Index clobbers votes for every lower index.
// Similarly, if a vote for an Index is casted and an existing vote for an higher Index exists, the operation has no effect.
type LatestVotes[EntityID constraints.Integer, VotePowerType constraints.Comparable[VotePowerType]] struct {
	voter *validator.Validator
	t     *thresholdmap.ThresholdMap[EntityID, VotePowerType]

	m sync.RWMutex
}

// NewLatestVotes creates a new NewLatestVotes instance associated with the given details.
func NewLatestVotes[EntityID constraints.Integer, VotePowerType constraints.Comparable[VotePowerType]](voter *validator.Validator) (newLatestMarkerVotes *LatestVotes[EntityID, VotePowerType]) {
	return &LatestVotes[EntityID, VotePowerType]{
		voter: voter,
		t:     thresholdmap.New[EntityID, VotePowerType](thresholdmap.UpperThresholdMode),
	}
}

// ForEach provides a callback based iterator that iterates through all Elements in the map.
func (l *LatestVotes[EntityID, VotePowerType]) ForEach(iterator func(node *thresholdmap.Element[EntityID, VotePowerType]) bool) {
	l.t.ForEach(iterator)
}

// Voter returns the Voter for the LatestVotes.
func (l *LatestVotes[EntityID, VotePowerType]) Voter() *validator.Validator {
	l.m.RLock()
	defer l.m.RUnlock()

	return l.voter
}

// Power returns the power of the vote for the given marker Index.
func (l *LatestVotes[EntityID, VotePowerType]) Power(index EntityID) (power VotePowerType, exists bool) {
	l.m.RLock()
	defer l.m.RUnlock()

	return l.t.Get(index)
}

// Store stores the vote with the given marker Index and votePower.
// The votePower parameter is used to determine the order of the vote.
func (l *LatestVotes[EntityID, VotePowerType]) Store(index EntityID, power VotePowerType) (stored bool, previousHighestIndex EntityID) {
	l.m.Lock()
	defer l.m.Unlock()
	if maxElement := l.t.MaxElement(); maxElement != nil {
		previousHighestIndex = maxElement.Key()
	}

	// abort if we already have a higher value on an Index that is larger or equal
	_, ceilingValue, ceilingExists := l.t.Ceiling(index)
	if ceilingExists && power.Compare(ceilingValue) <= 0 {
		return false, previousHighestIndex
	}

	// set the new value
	l.t.Set(index, power)

	// remove all predecessors that are lower than the newly set value
	floorKey, floorValue, floorExists := l.t.Floor(index - 1)
	for floorExists && floorValue.Compare(power) <= 0 {
		l.t.Delete(floorKey)

		floorKey, floorValue, floorExists = l.t.Floor(index - 1)
	}

	return true, previousHighestIndex
}
