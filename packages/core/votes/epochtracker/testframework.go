package epochtracker

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	EpochTracker *EpochTracker

	test *testing.T

	*VotesTestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework[VotePowerType constraints.Comparable[VotePowerType]](test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.VotesTestFramework == nil {
			t.VotesTestFramework = votes.NewTestFramework(test, votes.WithValidators(
				sybilprotection.NewWeights(mapdb.NewMapDB(), permanent.NewSettings(test.TempDir()+"/settings")).WeightedSet(),
			))
		}

		if t.EpochTracker == nil {
			t.EpochTracker = NewEpochTracker(func() epoch.Index { return 0 })
		}

		t.EpochTracker.Events.VotersUpdated.Hook(event.NewClosure(func(evt *VoterUpdatedEvent) {
			if debug.GetEnabled() {
				t.test.Logf("VOTER ADDED: %v", evt.NewLatestEpochIndex.String())
			}
		}))
	})
}

func (t *TestFramework) ValidateEpochVoters(expectedVoters map[epoch.Index]*set.AdvancedSet[identity.ID]) {
	for epochIndex, expectedVotersEpoch := range expectedVoters {
		voters := t.EpochTracker.Voters(epochIndex)

		assert.True(t.test, expectedVotersEpoch.Equal(voters), "epoch %s expected %s voters but got %s", epochIndex, expectedVotersEpoch, voters)
	}
}

type VotesTestFramework = votes.TestFramework

type MarkersTestFramework = markers.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithVotesTestFramework[VotePowerType constraints.Comparable[VotePowerType]](votesTestFramework *votes.TestFramework) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.VotesTestFramework != nil {
			panic("VotesTestFramework already set")
		}

		tf.VotesTestFramework = votesTestFramework
	}
}

func WithEpochTracker[VotePowerType constraints.Comparable[VotePowerType]](sequenceTracker *EpochTracker) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.EpochTracker != nil {
			panic("sequence tracker already set")
		}
		tf.EpochTracker = sequenceTracker
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
