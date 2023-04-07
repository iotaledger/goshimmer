package sequencetracker

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
)

// TestSequenceTracker_TrackVotes tests the SequenceTracker's functionality regarding tracking sequence votes.
func TestSequenceTracker_TrackVotes(t *testing.T) {
	debug.SetEnabled(true)

	votesTF := votes.NewTestFramework(t, sybilprotection.NewWeights(mapdb.NewMapDB()).NewWeightedSet())
	sequenceManager := markers.NewSequenceManager()
	sequenceTracker := NewSequenceTracker[votes.MockedVotePower](votesTF.Validators, sequenceManager.Sequence, func(sequenceID markers.SequenceID) markers.Index { return 1 })
	tf := NewTestFramework(t, votesTF, sequenceTracker, sequenceManager)

	tf.Votes.CreateValidator("A", 1)
	tf.Votes.CreateValidator("B", 1)

	// build markers DAG
	{
		tf.Markers.InheritStructureDetails("0,1", nil)
		tf.Markers.InheritStructureDetails("0,2", tf.Markers.StructureDetailsSet("0,1"))
		tf.Markers.InheritStructureDetails("0,3", tf.Markers.StructureDetailsSet("0,2"))
		tf.Markers.InheritStructureDetails("0,4", tf.Markers.StructureDetailsSet("0,3"))

		tf.Markers.StructureDetails("0,1").SetPastMarkerGap(50)

		tf.Markers.InheritStructureDetails("1,2", tf.Markers.StructureDetailsSet("0,1"))
		tf.Markers.InheritStructureDetails("1,3", tf.Markers.StructureDetailsSet("1,2"))
		tf.Markers.InheritStructureDetails("1,4", tf.Markers.StructureDetailsSet("1,3"))
		tf.Markers.InheritStructureDetails("1,5", tf.Markers.StructureDetailsSet("1,4"))

		tf.Markers.StructureDetails("0,3").SetPastMarkerGap(50)
		tf.Markers.StructureDetails("1,4").SetPastMarkerGap(50)

		tf.Markers.InheritStructureDetails("2,5", tf.Markers.StructureDetailsSet("0,3", "1,4"))
		tf.Markers.InheritStructureDetails("2,6", tf.Markers.StructureDetailsSet("0,4", "2,5"))
		tf.Markers.InheritStructureDetails("2,7", tf.Markers.StructureDetailsSet("2,6"))
		tf.Markers.InheritStructureDetails("2,8", tf.Markers.StructureDetailsSet("2,7"))

		tf.Markers.StructureDetails("2,7").SetPastMarkerGap(50)

		tf.Markers.InheritStructureDetails("3,8", tf.Markers.StructureDetailsSet("2,7"))

		tf.Markers.StructureDetails("1,4").SetPastMarkerGap(50)

		tf.Markers.InheritStructureDetails("4,8", tf.Markers.StructureDetailsSet("2,7", "1,4"))
	}

	expectedVoters := map[string]*advancedset.AdvancedSet[identity.ID]{
		"0,1": tf.Votes.ValidatorsSet(),
		"0,2": tf.Votes.ValidatorsSet(),
		"0,3": tf.Votes.ValidatorsSet(),
		"0,4": tf.Votes.ValidatorsSet(),
		"1,2": tf.Votes.ValidatorsSet(),
		"1,3": tf.Votes.ValidatorsSet(),
		"1,4": tf.Votes.ValidatorsSet(),
		"1,5": tf.Votes.ValidatorsSet(),
		"2,5": tf.Votes.ValidatorsSet(),
		"2,6": tf.Votes.ValidatorsSet(),
		"2,7": tf.Votes.ValidatorsSet(),
		"2,8": tf.Votes.ValidatorsSet(),
		"3,8": tf.Votes.ValidatorsSet(),
		"4,8": tf.Votes.ValidatorsSet(),
	}
	// CASE1: APPROVE MARKER(0, 3)
	{
		tf.Instance.TrackVotes(markers.NewMarkers(markers.NewMarker(0, 3)), tf.Votes.Validator("A"), votes.MockedVotePower{VotePower: 0})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*advancedset.AdvancedSet[identity.ID]{
			"0,1": tf.Votes.ValidatorsSet("A"),
			"0,2": tf.Votes.ValidatorsSet("A"),
			"0,3": tf.Votes.ValidatorsSet("A"),
		}))
	}
	// CASE2: APPROVE MARKER(0, 4) + MARKER(2, 6)
	{
		tf.Instance.TrackVotes(markers.NewMarkers(markers.NewMarker(0, 4), markers.NewMarker(2, 6)), tf.Votes.Validator("A"), votes.MockedVotePower{VotePower: 1})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*advancedset.AdvancedSet[identity.ID]{
			"0,4": tf.Votes.ValidatorsSet("A"),
			"1,2": tf.Votes.ValidatorsSet("A"),
			"1,3": tf.Votes.ValidatorsSet("A"),
			"1,4": tf.Votes.ValidatorsSet("A"),
			"2,5": tf.Votes.ValidatorsSet("A"),
			"2,6": tf.Votes.ValidatorsSet("A"),
		}))
	}

	// CASE3: APPROVE MARKER(4, 8)
	{
		tf.Instance.TrackVotes(markers.NewMarkers(markers.NewMarker(4, 8)), tf.Votes.Validator("A"), votes.MockedVotePower{VotePower: 2})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*advancedset.AdvancedSet[identity.ID]{
			"2,7": tf.Votes.ValidatorsSet("A"),
			"4,8": tf.Votes.ValidatorsSet("A"),
		}))
	}

	// CASE4: APPROVE MARKER(1, 5)
	{
		tf.Instance.TrackVotes(markers.NewMarkers(markers.NewMarker(1, 5)), tf.Votes.Validator("B"), votes.MockedVotePower{VotePower: 3})

		tf.ValidateStructureDetailsVoters(lo.MergeMaps(expectedVoters, map[string]*advancedset.AdvancedSet[identity.ID]{
			"0,1": tf.Votes.ValidatorsSet("A", "B"),
			"1,2": tf.Votes.ValidatorsSet("A", "B"),
			"1,3": tf.Votes.ValidatorsSet("A", "B"),
			"1,4": tf.Votes.ValidatorsSet("A", "B"),
			"1,5": tf.Votes.ValidatorsSet("B"),
		}))
	}
}
