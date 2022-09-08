package sequencetracker

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/thresholdmap"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
)

// LatestMarkerVotes keeps track of the most up-to-date for a certain Voter casted on a specific Marker SequenceID.
// Votes can be casted on Markers (SequenceID, Index), but can arrive in any arbitrary order.
// Due to the nature of a Sequence, a vote casted for a certain Index clobbers votes for every lower index.
// Similarly, if a vote for an Index is casted and an existing vote for an higher Index exists, the operation has no effect.
type LatestMarkerVotes[VotePowerType votes.VotePower[VotePowerType]] struct {
	voter *validator.Validator
	t     thresholdmap.ThresholdMap[markers.Index, VotePowerType]

	m sync.RWMutex
}

// NewLatestMarkerVotes creates a new NewLatestMarkerVotes instance associated with the given details.
func NewLatestMarkerVotes[VotePowerType votes.VotePower[VotePowerType]](voter *validator.Validator) (newLatestMarkerVotes *LatestMarkerVotes[VotePowerType]) {
	return &LatestMarkerVotes[VotePowerType]{
		voter: voter,
		t:     *thresholdmap.New[markers.Index, VotePowerType](thresholdmap.UpperThresholdMode),
	}
}

// Voter returns the Voter for the LatestMarkerVotes.
func (l *LatestMarkerVotes[VotePowerType]) Voter() *validator.Validator {
	l.m.RLock()
	defer l.m.RUnlock()

	return l.voter
}

// Power returns the power of the vote for the given marker Index.
func (l *LatestMarkerVotes[VotePowerType]) Power(index markers.Index) (power VotePowerType, exists bool) {
	l.m.RLock()
	defer l.m.RUnlock()

	return l.t.Get(index)
}

// Store stores the vote with the given marker Index and votePower.
// The votePower parameter is used to determine the order of the vote.
func (l *LatestMarkerVotes[VotePowerType]) Store(index markers.Index, power VotePowerType) (stored bool, previousHighestIndex markers.Index) {
	l.m.Lock()
	defer l.m.Unlock()
	if maxElement := l.t.MaxElement(); maxElement != nil {
		previousHighestIndex = maxElement.Key()
	}

	// abort if we already have a higher value on an Index that is larger or equal
	_, ceilingValue, ceilingExists := l.t.Ceiling(index)
	if ceilingExists && power.CompareTo(ceilingValue) <= 0 {
		return false, previousHighestIndex
	}

	// set the new value
	l.t.Set(index, power)

	// remove all predecessors that are lower than the newly set value
	floorKey, floorValue, floorExists := l.t.Floor(index - 1)
	for floorExists && floorValue.CompareTo(power) <= 0 {
		l.t.Delete(floorKey)

		floorKey, floorValue, floorExists = l.t.Floor(index - 1)
	}

	return true, previousHighestIndex
}
