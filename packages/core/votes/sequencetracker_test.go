package votes

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/thresholdmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// TestApprovalWeightManager_updateSequenceVoters tests the ApprovalWeightManager's functionality regarding sequences.
// The scenario can be found in images/approvalweight-updateSequenceSupporters.png.
func TestApprovalWeightManager_updateSequenceVoters(t *testing.T) {
	debug.SetEnabled(true)
	tf := NewTestFramework(t)

	tf.CreateValidator("A")
	tf.CreateValidator("B")

	// build markers DAG
	{
		tf.InheritStructureDetails("0,1", nil)
		tf.InheritStructureDetails("0,2", tf.StructureDetailsSet("0,1"))
		tf.InheritStructureDetails("0,3", tf.StructureDetailsSet("0,2"))
		tf.InheritStructureDetails("0,4", tf.StructureDetailsSet("0,3"))

		tf.StructureDetails("0,1").SetPastMarkerGap(50)

		tf.InheritStructureDetails("1,2", tf.StructureDetailsSet("0,1"))
		tf.InheritStructureDetails("1,3", tf.StructureDetailsSet("1,2"))
		tf.InheritStructureDetails("1,4", tf.StructureDetailsSet("1,3"))
		tf.InheritStructureDetails("1,5", tf.StructureDetailsSet("1,4"))

		tf.StructureDetails("0,3").SetPastMarkerGap(50)
		tf.StructureDetails("1,4").SetPastMarkerGap(50)

		tf.InheritStructureDetails("2,5", tf.StructureDetailsSet("0,3", "1,4"))
		tf.InheritStructureDetails("2,6", tf.StructureDetailsSet("0,4", "2,5"))
		tf.InheritStructureDetails("2,7", tf.StructureDetailsSet("2,6"))
		tf.InheritStructureDetails("2,8", tf.StructureDetailsSet("2,7"))

		tf.StructureDetails("2,7").SetPastMarkerGap(50)

		tf.InheritStructureDetails("3,8", tf.StructureDetailsSet("2,7"))

		tf.StructureDetails("1,4").SetPastMarkerGap(50)

		tf.InheritStructureDetails("4,8", tf.StructureDetailsSet("2,7", "1,4"))
	}

	expectedVoters := map[string]*set.AdvancedSet[*validator.Validator]{
		"0,1": tf.Validators(),
		"0,2": tf.Validators(),
		"0,3": tf.Validators(),
		"0,4": tf.Validators(),
		"1,2": tf.Validators(),
		"1,3": tf.Validators(),
		"1,4": tf.Validators(),
		"1,5": tf.Validators(),
		"2,5": tf.Validators(),
		"2,6": tf.Validators(),
		"2,7": tf.Validators(),
		"2,8": tf.Validators(),
		"3,8": tf.Validators(),
		"4,8": tf.Validators(),
	}
	// CASE1: APPROVE MARKER(0, 3)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(0, 3)), tf.Validator("A").ID(), mockVotePower{0})

		tf.validateMarkerVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[*validator.Validator]{
			"0,1": tf.Validators("A"),
			"0,2": tf.Validators("A"),
			"0,3": tf.Validators("A"),
		}))
	}
	// CASE2: APPROVE MARKER(0, 4) + MARKER(2, 6)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(0, 4), markers.NewMarker(2, 6)), tf.Validator("A").ID(), mockVotePower{1})

		tf.validateMarkerVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[*validator.Validator]{
			"0,4": tf.Validators("A"),
			"1,2": tf.Validators("A"),
			"1,3": tf.Validators("A"),
			"1,4": tf.Validators("A"),
			"2,5": tf.Validators("A"),
			"2,6": tf.Validators("A"),
		}))
	}

	// CASE3: APPROVE MARKER(4, 8)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(4, 8)), tf.Validator("A").ID(), mockVotePower{2})

		tf.validateMarkerVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[*validator.Validator]{
			"2,7": tf.Validators("A"),
			"4,8": tf.Validators("A"),
		}))
	}

	// CASE4: APPROVE MARKER(1, 5)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(1, 5)), tf.Validator("B").ID(), mockVotePower{3})

		tf.validateMarkerVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[*validator.Validator]{
			"0,1": tf.Validators("A", "B"),
			"1,2": tf.Validators("A", "B"),
			"1,3": tf.Validators("A", "B"),
			"1,4": tf.Validators("A", "B"),
			"1,5": tf.Validators("B"),
		}))
	}
}

func TestLatestMarkerVotes(t *testing.T) {
	voter := validator.New(identity.ID{})

	{
		latestMarkerVotes := NewLatestMarkerVotes[mockVotePower](voter)
		latestMarkerVotes.Store(1, mockVotePower{8})
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]mockVotePower{
			1: {8},
		})
		latestMarkerVotes.Store(2, mockVotePower{10})
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]mockVotePower{
			2: {10},
		})
		latestMarkerVotes.Store(3, mockVotePower{7})
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]mockVotePower{
			2: {10},
			3: {7},
		})
		latestMarkerVotes.Store(4, mockVotePower{9})
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]mockVotePower{
			2: {10},
			4: {9},
		})
		latestMarkerVotes.Store(4, mockVotePower{11})
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]mockVotePower{
			4: {11},
		})
		latestMarkerVotes.Store(1, mockVotePower{15})
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]mockVotePower{
			1: {15},
			4: {11},
		})
	}

	{
		latestMarkerVotes := NewLatestMarkerVotes[mockVotePower](voter)
		latestMarkerVotes.Store(3, mockVotePower{7})
		latestMarkerVotes.Store(2, mockVotePower{10})
		latestMarkerVotes.Store(4, mockVotePower{9})
		latestMarkerVotes.Store(1, mockVotePower{8})
		latestMarkerVotes.Store(1, mockVotePower{15})
		latestMarkerVotes.Store(4, mockVotePower{11})
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]mockVotePower{
			1: {15},
			4: {11},
		})
	}
}

func validateLatestMarkerVotes[VotePowerType VotePower[VotePowerType]](t *testing.T, votes *LatestMarkerVotes[VotePowerType], expectedVotes map[markers.Index]VotePowerType) {
	votes.t.ForEach(func(node *thresholdmap.Element[markers.Index, VotePowerType]) bool {
		index := node.Key()
		votePower := node.Value()

		expectedVotePower, exists := expectedVotes[index]
		assert.Truef(t, exists, "%s does not exist in latestMarkerVotes", index)
		delete(expectedVotes, index)

		assert.Equal(t, expectedVotePower, votePower)

		return true
	})
	assert.Empty(t, expectedVotes)
}
