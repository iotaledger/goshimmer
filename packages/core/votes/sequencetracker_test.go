package votes

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/thresholdmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

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
