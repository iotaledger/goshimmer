package slottracker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/runtime/debug"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test        *testing.T
	SlotTracker *SlotTracker

	Votes *votes.TestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, slotTracker *SlotTracker, votesTF *votes.TestFramework) *TestFramework {
	t := &TestFramework{
		test:        test,
		SlotTracker: slotTracker,
		Votes:       votesTF,
	}

	t.SlotTracker.Events.VotersUpdated.Hook(func(evt *VoterUpdatedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("VOTER ADDED: %v", evt.NewLatestSlotIndex.String())
		}
	})

	return t
}

func NewDefaultTestFramework(t *testing.T) *TestFramework {
	return NewTestFramework(t, NewSlotTracker(func() slot.Index { return 0 }),
		votes.NewTestFramework(t,
			sybilprotection.NewWeights(mapdb.NewMapDB()).NewWeightedSet(),
		),
	)
}

func (t *TestFramework) ValidateSlotVoters(expectedVoters map[slot.Index]*advancedset.AdvancedSet[identity.ID]) {
	for slotIndex, expectedVotersSlot := range expectedVoters {
		voters := t.SlotTracker.Voters(slotIndex)

		assert.True(t.test, expectedVotersSlot.Equal(voters), "slot %s expected %s voters but got %s", slotIndex, expectedVotersSlot, voters)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
