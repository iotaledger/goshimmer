package latestvotes

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/thresholdmap"
)

func TestLatestVotes(t *testing.T) {
	voter := identity.ID{}

	{
		latestVotes := NewLatestVotes[markers.Index, votes.MockedVotePower](voter)
		latestVotes.Store(1, votes.MockedVotePower{VotePower: 8})
		validateLatestVotes(t, latestVotes, map[markers.Index]votes.MockedVotePower{
			1: {VotePower: 8},
		})
		latestVotes.Store(2, votes.MockedVotePower{VotePower: 10})
		validateLatestVotes(t, latestVotes, map[markers.Index]votes.MockedVotePower{
			2: {VotePower: 10},
		})
		latestVotes.Store(3, votes.MockedVotePower{VotePower: 7})
		validateLatestVotes(t, latestVotes, map[markers.Index]votes.MockedVotePower{
			2: {VotePower: 10},
			3: {VotePower: 7},
		})
		latestVotes.Store(4, votes.MockedVotePower{VotePower: 9})
		validateLatestVotes(t, latestVotes, map[markers.Index]votes.MockedVotePower{
			2: {VotePower: 10},
			4: {VotePower: 9},
		})
		latestVotes.Store(4, votes.MockedVotePower{VotePower: 11})
		validateLatestVotes(t, latestVotes, map[markers.Index]votes.MockedVotePower{
			4: {VotePower: 11},
		})
		latestVotes.Store(1, votes.MockedVotePower{VotePower: 15})
		validateLatestVotes(t, latestVotes, map[markers.Index]votes.MockedVotePower{
			1: {VotePower: 15},
			4: {VotePower: 11},
		})
	}

	{
		latestVotes := NewLatestVotes[markers.Index, votes.MockedVotePower](voter)
		latestVotes.Store(3, votes.MockedVotePower{VotePower: 7})
		latestVotes.Store(2, votes.MockedVotePower{VotePower: 10})
		latestVotes.Store(4, votes.MockedVotePower{VotePower: 9})
		latestVotes.Store(1, votes.MockedVotePower{VotePower: 8})
		latestVotes.Store(1, votes.MockedVotePower{VotePower: 15})
		latestVotes.Store(4, votes.MockedVotePower{VotePower: 11})
		validateLatestVotes(t, latestVotes, map[markers.Index]votes.MockedVotePower{
			1: {VotePower: 15},
			4: {VotePower: 11},
		})
	}
}

func validateLatestVotes[VotePowerType constraints.Comparable[VotePowerType]](t *testing.T, votes *LatestVotes[markers.Index, VotePowerType], expectedVotes map[markers.Index]VotePowerType) {
	votes.ForEach(func(node *thresholdmap.Element[markers.Index, VotePowerType]) bool {
		index := node.Key()
		votePower := node.Value()

		expectedVotePower, exists := expectedVotes[index]
		assert.Truef(t, exists, "%s does not exist in latestVotes", index)
		delete(expectedVotes, index)

		assert.Equal(t, expectedVotePower, votePower)

		return true
	})
	assert.Empty(t, expectedVotes)
}
