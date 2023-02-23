package epochtracker

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
)

func TestEpochTracker_TrackVotes(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewDefaultTestFramework(t)

	tf.Votes.CreateValidator("A", 1)
	tf.Votes.CreateValidator("B", 1)

	expectedVoters := map[epoch.Index]*advancedset.AdvancedSet[identity.ID]{
		1: tf.Votes.ValidatorsSet(),
		2: tf.Votes.ValidatorsSet(),
		3: tf.Votes.ValidatorsSet(),
		4: tf.Votes.ValidatorsSet(),
		5: tf.Votes.ValidatorsSet(),
		6: tf.Votes.ValidatorsSet(),
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(1), tf.Votes.Validator("A"), EpochVotePower{6})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*advancedset.AdvancedSet[identity.ID]{
			1: tf.Votes.ValidatorsSet("A"),
		}))
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(2), tf.Votes.Validator("A"), EpochVotePower{7})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*advancedset.AdvancedSet[identity.ID]{
			2: tf.Votes.ValidatorsSet("A"),
		}))
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(5), tf.Votes.Validator("A"), EpochVotePower{11})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*advancedset.AdvancedSet[identity.ID]{
			3: tf.Votes.ValidatorsSet("A"),
			4: tf.Votes.ValidatorsSet("A"),
			5: tf.Votes.ValidatorsSet("A"),
		}))
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(6), tf.Votes.Validator("B"), EpochVotePower{12})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*advancedset.AdvancedSet[identity.ID]{
			1: tf.Votes.ValidatorsSet("A", "B"),
			2: tf.Votes.ValidatorsSet("A", "B"),
			3: tf.Votes.ValidatorsSet("A", "B"),
			4: tf.Votes.ValidatorsSet("A", "B"),
			5: tf.Votes.ValidatorsSet("A", "B"),
			6: tf.Votes.ValidatorsSet("B"),
		}))
	}
}
