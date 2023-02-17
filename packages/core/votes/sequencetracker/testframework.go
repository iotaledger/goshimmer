package sequencetracker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework[VotePowerType constraints.Comparable[VotePowerType]] struct {
	test            *testing.T
	Instance        *SequenceTracker[VotePowerType]
	SequenceManager *markers.SequenceManager

	Votes   *votes.TestFramework
	Markers *markers.TestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework[VotePowerType constraints.Comparable[VotePowerType]](test *testing.T, votesTF *votes.TestFramework, sequenceTracker *SequenceTracker[VotePowerType], sequenceManager *markers.SequenceManager) *TestFramework[VotePowerType] {
	t := &TestFramework[VotePowerType]{
		test:            test,
		Votes:           votesTF,
		Instance:        sequenceTracker,
		SequenceManager: sequenceManager,
		Markers:         markers.NewTestFramework(test, markers.WithSequenceManager(sequenceManager)),
	}

	event.Hook(t.Instance.Events.VotersUpdated, func(evt *VoterUpdatedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("VOTER ADDED: %v, %v", markers.NewMarker(evt.SequenceID, evt.NewMaxSupportedIndex), evt.Voter)
		}
	})
	return t
}

func (t *TestFramework[VotePowerType]) ValidateStructureDetailsVoters(expectedVoters map[string]*set.AdvancedSet[identity.ID]) {
	for markerAlias, expectedVotersOfMarker := range expectedVoters {
		// sanity check
		assert.Equal(t.test, markerAlias, fmt.Sprintf("%d,%d", t.Markers.StructureDetails(markerAlias).PastMarkers().Marker().SequenceID(), t.Markers.StructureDetails(markerAlias).PastMarkers().Marker().Index()))

		voters := t.Instance.Voters(t.Markers.StructureDetails(markerAlias).PastMarkers().Marker())

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", markerAlias, expectedVotersOfMarker.Size(), voters.Size())
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
