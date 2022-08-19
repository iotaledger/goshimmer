package votes

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/thresholdmap"

	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// LatestMarkerVotes keeps track of the most up-to-date for a certain Voter casted on a specific Marker SequenceID.
// Votes can be casted on Markers (SequenceID, Index), but can arrive in any arbitrary order.
// Due to the nature of a Sequence, a vote casted for a certain Index clobbers votes for every lower index.
// Similarly, if a vote for an Index is casted and an existing vote for an higher Index exists, the operation has no effect.
type LatestMarkerVotes struct {
	voter *validator.Validator
	t     thresholdmap.ThresholdMap[markers.Index, VotePower]

	m sync.RWMutex
}

// NewLatestMarkerVotes creates a new NewLatestMarkerVotes instance associated with the given details.
func NewLatestMarkerVotes(voter *validator.Validator, sequenceID markers.SequenceID) (newLatestMarkerVotes *LatestMarkerVotes) {
	return &LatestMarkerVotes{
		voter: voter,
		t:     thresholdmap.ThresholdMap[markers.Index, VotePower]{},
	}
}

// Voter returns the Voter for the LatestMarkerVotes.
func (l *LatestMarkerVotes) Voter() *validator.Validator {
	l.t.RLock()
	defer l.t.RUnlock()

	return l.voter
}

// Power returns the power of the vote for the given marker Index.
func (l *LatestMarkerVotes) Power(index markers.Index) (power VotePower, exists bool) {
	l.t.RLock()
	defer l.t.RUnlock()

	key, exists := l.t.Get(index)
	if !exists {
		return nil, exists
	}

	return key, exists
}

// Store stores the vote with the given marker Index and votePower.
// The votePower parameter is used to determine the order of the vote.
func (l *LatestMarkerVotes) Store(index markers.Index, power VotePower) (stored bool, previousHighestIndex markers.Index) {
	l.t.Lock()
	defer l.t.Unlock()

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
