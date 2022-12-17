package sequencetracker

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
)

// TestSequenceTracker_TrackVotes tests the SequenceTracker's functionality regarding tracking sequence votes.
func TestSequenceTracker_TrackVotes(t *testing.T) {
	debug.SetEnabled(true)
	tf := NewTestFramework[votes.MockedVotePower](t)

	tf.CreateValidator("A", 1)
	tf.CreateValidator("B", 1)

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

	expectedVoters := map[string]*set.AdvancedSet[identity.ID]{
		"0,1": tf.ValidatorsSet(),
		"0,2": tf.ValidatorsSet(),
		"0,3": tf.ValidatorsSet(),
		"0,4": tf.ValidatorsSet(),
		"1,2": tf.ValidatorsSet(),
		"1,3": tf.ValidatorsSet(),
		"1,4": tf.ValidatorsSet(),
		"1,5": tf.ValidatorsSet(),
		"2,5": tf.ValidatorsSet(),
		"2,6": tf.ValidatorsSet(),
		"2,7": tf.ValidatorsSet(),
		"2,8": tf.ValidatorsSet(),
		"3,8": tf.ValidatorsSet(),
		"4,8": tf.ValidatorsSet(),
	}
	// CASE1: APPROVE MARKER(0, 3)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(0, 3)), tf.Validator("A"), votes.MockedVotePower{VotePower: 0})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[identity.ID]{
			"0,1": tf.ValidatorsSet("A"),
			"0,2": tf.ValidatorsSet("A"),
			"0,3": tf.ValidatorsSet("A"),
		}))
	}
	// CASE2: APPROVE MARKER(0, 4) + MARKER(2, 6)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(0, 4), markers.NewMarker(2, 6)), tf.Validator("A"), votes.MockedVotePower{VotePower: 1})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[identity.ID]{
			"0,4": tf.ValidatorsSet("A"),
			"1,2": tf.ValidatorsSet("A"),
			"1,3": tf.ValidatorsSet("A"),
			"1,4": tf.ValidatorsSet("A"),
			"2,5": tf.ValidatorsSet("A"),
			"2,6": tf.ValidatorsSet("A"),
		}))
	}

	// CASE3: APPROVE MARKER(4, 8)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(4, 8)), tf.Validator("A"), votes.MockedVotePower{VotePower: 2})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[identity.ID]{
			"2,7": tf.ValidatorsSet("A"),
			"4,8": tf.ValidatorsSet("A"),
		}))
	}

	// CASE4: APPROVE MARKER(1, 5)
	{
		tf.SequenceTracker.TrackVotes(markers.NewMarkers(markers.NewMarker(1, 5)), tf.Validator("B"), votes.MockedVotePower{VotePower: 3})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*set.AdvancedSet[identity.ID]{
			"0,1": tf.ValidatorsSet("A", "B"),
			"1,2": tf.ValidatorsSet("A", "B"),
			"1,3": tf.ValidatorsSet("A", "B"),
			"1,4": tf.ValidatorsSet("A", "B"),
			"1,5": tf.ValidatorsSet("B"),
		}))
	}
}
