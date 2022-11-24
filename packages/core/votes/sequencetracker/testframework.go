package sequencetracker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework[VotePowerType constraints.Comparable[VotePowerType]] struct {
	SequenceTracker *SequenceTracker[VotePowerType]
	sequenceManager *markers.SequenceManager

	test *testing.T

	*VotesTestFramework
	*MarkersTestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework[VotePowerType constraints.Comparable[VotePowerType]](test *testing.T, opts ...options.Option[TestFramework[VotePowerType]]) (newTestFramework *TestFramework[VotePowerType]) {
	return options.Apply(&TestFramework[VotePowerType]{
		test: test,
	}, opts, func(t *TestFramework[VotePowerType]) {
		if t.VotesTestFramework == nil {
			t.VotesTestFramework = votes.NewTestFramework(test, votes.WithValidators(
				sybilprotection.NewWeights(mapdb.NewMapDB(), permanent.NewSettings(test.TempDir()+"/settings")).WeightedSet(),
			))
		}

		t.MarkersTestFramework = markers.NewTestFramework(t.test, markers.WithSequenceManager(t.sequenceManager))

		if t.SequenceTracker == nil {
			t.SequenceTracker = NewSequenceTracker[VotePowerType](t.VotesTestFramework.Validators, t.SequenceManager().Sequence, func(sequenceID markers.SequenceID) markers.Index { return 1 })
		}

		t.SequenceTracker.Events.VotersUpdated.Hook(event.NewClosure(func(evt *VoterUpdatedEvent) {
			if debug.GetEnabled() {
				t.test.Logf("VOTER ADDED: %v", markers.NewMarker(evt.SequenceID, evt.NewMaxSupportedIndex))
			}
		}))
	})
}

func (t *TestFramework[VotePowerType]) ValidateStructureDetailsVoters(expectedVoters map[string]*set.AdvancedSet[identity.ID]) {
	for markerAlias, expectedVotersOfMarker := range expectedVoters {
		// sanity check
		assert.Equal(t.test, markerAlias, fmt.Sprintf("%d,%d", t.StructureDetails(markerAlias).PastMarkers().Marker().SequenceID(), t.StructureDetails(markerAlias).PastMarkers().Marker().Index()))

		voters := t.SequenceTracker.Voters(t.StructureDetails(markerAlias).PastMarkers().Marker())

		assert.True(t.test, expectedVotersOfMarker.Equal(voters.Members()), "marker %s expected %d voters but got %d", markerAlias, expectedVotersOfMarker.Size(), voters.Members().Size())
	}
}

type VotesTestFramework = votes.TestFramework

type MarkersTestFramework = markers.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithVotesTestFramework[VotePowerType constraints.Comparable[VotePowerType]](votesTestFramework *votes.TestFramework) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.VotesTestFramework != nil {
			panic("VotesTestFramework already set")
		}

		tf.VotesTestFramework = votesTestFramework
	}
}

func WithSequenceTracker[VotePowerType constraints.Comparable[VotePowerType]](sequenceTracker *SequenceTracker[VotePowerType]) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.SequenceTracker != nil {
			panic("sequence tracker already set")
		}
		tf.SequenceTracker = sequenceTracker
	}
}

func WithSequenceManager[VotePowerType constraints.Comparable[VotePowerType]](sequenceManager *markers.SequenceManager) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.sequenceManager != nil {
			panic("sequence manager already set")
		}
		tf.sequenceManager = sequenceManager
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
