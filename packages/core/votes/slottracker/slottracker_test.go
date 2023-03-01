package slottracker

import (
	"testing"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
)

func TestSlotTracker_TrackVotes(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewDefaultTestFramework(t)

	tf.Votes.CreateValidator("A", 1)
	tf.Votes.CreateValidator("B", 1)

	expectedVoters := map[slot.Index]*advancedset.AdvancedSet[identity.ID]{
		1: tf.Votes.ValidatorsSet(),
		2: tf.Votes.ValidatorsSet(),
		3: tf.Votes.ValidatorsSet(),
		4: tf.Votes.ValidatorsSet(),
		5: tf.Votes.ValidatorsSet(),
		6: tf.Votes.ValidatorsSet(),
	}

	{
		tf.SlotTracker.TrackVotes(slot.Index(1), tf.Votes.Validator("A"), SlotVotePower{6})

		tf.ValidateSlotVoters(lo.MergeMaps(expectedVoters, map[slot.Index]*advancedset.AdvancedSet[identity.ID]{
			1: tf.Votes.ValidatorsSet("A"),
		}))
	}

	{
		tf.SlotTracker.TrackVotes(slot.Index(2), tf.Votes.Validator("A"), SlotVotePower{7})

		tf.ValidateSlotVoters(lo.MergeMaps(expectedVoters, map[slot.Index]*advancedset.AdvancedSet[identity.ID]{
			2: tf.Votes.ValidatorsSet("A"),
		}))
	}

	{
		tf.SlotTracker.TrackVotes(slot.Index(5), tf.Votes.Validator("A"), SlotVotePower{11})

		tf.ValidateSlotVoters(lo.MergeMaps(expectedVoters, map[slot.Index]*advancedset.AdvancedSet[identity.ID]{
			3: tf.Votes.ValidatorsSet("A"),
			4: tf.Votes.ValidatorsSet("A"),
			5: tf.Votes.ValidatorsSet("A"),
		}))
	}

	{
		tf.SlotTracker.TrackVotes(slot.Index(6), tf.Votes.Validator("B"), SlotVotePower{12})

		tf.ValidateSlotVoters(lo.MergeMaps(expectedVoters, map[slot.Index]*advancedset.AdvancedSet[identity.ID]{
			1: tf.Votes.ValidatorsSet("A", "B"),
			2: tf.Votes.ValidatorsSet("A", "B"),
			3: tf.Votes.ValidatorsSet("A", "B"),
			4: tf.Votes.ValidatorsSet("A", "B"),
			5: tf.Votes.ValidatorsSet("A", "B"),
			6: tf.Votes.ValidatorsSet("B"),
		}))
	}
}
