package epochtracker

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

func TestEpochTracker_TrackVotes(t *testing.T) {
	debug.SetEnabled(true)
	tf := NewTestFramework[EpochVotePower](t)

	tf.CreateValidator("A")
	tf.CreateValidator("B")

	expectedVoters := map[epoch.Index]*set.AdvancedSet[*validator.Validator]{
		1: tf.Validators(),
		2: tf.Validators(),
		3: tf.Validators(),
		4: tf.Validators(),
		5: tf.Validators(),
		6: tf.Validators(),
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(1), tf.Validator("A").ID(), EpochVotePower{6})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*set.AdvancedSet[*validator.Validator]{
			1: tf.Validators("A"),
		}))
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(2), tf.Validator("A").ID(), EpochVotePower{7})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*set.AdvancedSet[*validator.Validator]{
			2: tf.Validators("A"),
		}))
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(5), tf.Validator("A").ID(), EpochVotePower{11})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*set.AdvancedSet[*validator.Validator]{
			3: tf.Validators("A"),
			4: tf.Validators("A"),
			5: tf.Validators("A"),
		}))
	}

	{
		tf.EpochTracker.TrackVotes(epoch.Index(6), tf.Validator("B").ID(), EpochVotePower{12})

		tf.ValidateEpochVoters(lo.MergeMaps(expectedVoters, map[epoch.Index]*set.AdvancedSet[*validator.Validator]{
			1: tf.Validators("A", "B"),
			2: tf.Validators("A", "B"),
			3: tf.Validators("A", "B"),
			4: tf.Validators("A", "B"),
			5: tf.Validators("A", "B"),
			6: tf.Validators("B"),
		}))
	}
}
