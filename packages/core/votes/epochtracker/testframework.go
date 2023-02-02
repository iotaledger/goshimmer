package epochtracker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test         *testing.T
	EpochTracker *EpochTracker

	Votes *votes.TestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, epochTracker *EpochTracker, votesTF *votes.TestFramework) *TestFramework {
	t := &TestFramework{
		test:         test,
		EpochTracker: epochTracker,
		Votes:        votesTF,
	}

	t.EpochTracker.Events.VotersUpdated.Hook(event.NewClosure(func(evt *VoterUpdatedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("VOTER ADDED: %v", evt.NewLatestEpochIndex.String())
		}
	}))

	return t
}

func NewDefaultTestFramework(t *testing.T) *TestFramework {
	return NewTestFramework(t, NewEpochTracker(func() epoch.Index { return 0 }),
		votes.NewTestFramework(t,
			sybilprotection.NewWeights(mapdb.NewMapDB()).NewWeightedSet(),
		),
	)
}

func (t *TestFramework) ValidateEpochVoters(expectedVoters map[epoch.Index]*set.AdvancedSet[identity.ID]) {
	for epochIndex, expectedVotersEpoch := range expectedVoters {
		voters := t.EpochTracker.Voters(epochIndex)

		assert.True(t.test, expectedVotersEpoch.Equal(voters), "epoch %s expected %s voters but got %s", epochIndex, expectedVotersEpoch, voters)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
