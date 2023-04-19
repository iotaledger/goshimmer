package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
)

func TestAll[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, frameworkProvider func(*testing.T) *Framework[ConflictID, ResourceID, VotePower]) {
	for testName, testCase := range map[string]func(*testing.T, *Framework[ConflictID, ResourceID, VotePower]){
		"CreateConflict":        CreateConflict[ConflictID, ResourceID, VotePower],
		"TestJoinConflictSets":  TestJoinConflictSets[ConflictID, ResourceID, VotePower],
		"UpdateConflictParents": UpdateConflictParents[ConflictID, ResourceID, VotePower],
		"LikedInstead":          LikedInstead[ConflictID, ResourceID, VotePower],
		"ConflictAcceptance":    ConflictAcceptance[ConflictID, ResourceID, VotePower],
		"CastVotes":             CastVotes[ConflictID, ResourceID, VotePower],
		"CastVotes_VotePower":   CastVotesVotePower[ConflictID, ResourceID, VotePower],
		"CastVotesAcceptance":   CastVotesAcceptance[ConflictID, ResourceID, VotePower],
	} {
		t.Run(testName, func(t *testing.T) { testCase(t, frameworkProvider(t)) })
	}
}

func TestJoinConflictSets[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	require.NoError(tf.test, tf.CreateConflict("conflict1", nil, []string{"resource1"}, acceptance.Pending))
	require.NoError(t, tf.CreateConflict("conflict2", nil, []string{"resource1"}, acceptance.Rejected))

	require.ErrorIs(t, tf.JoinConflictSets("conflict3", "resource1"), conflictdag.ErrEntityEvicted, "modifying non-existing conflicts should fail with ErrEntityEvicted")
	require.ErrorIs(t, tf.JoinConflictSets("conflict2", "resource2"), conflictdag.ErrEntityEvicted, "modifying rejected conflicts should fail with ErrEntityEvicted")

	require.NoError(t, tf.CreateConflict("conflict3", nil, []string{"resource2"}, acceptance.Pending))
	require.NoError(t, tf.JoinConflictSets("conflict1", "resource2"))
	tf.Assert.ConflictSetMembers("resource2", "conflict1", "conflict3")

	require.NoError(t, tf.JoinConflictSets("conflict2", "resource2"))
	tf.Assert.ConflictSetMembers("resource2", "conflict1", "conflict2", "conflict3")

	tf.Assert.LikedInstead([]string{"conflict3"}, "conflict1")
}

func UpdateConflictParents[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource2"}))

	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1", "conflict2"}, []string{"resource1", "resource2"}))
	tf.Assert.Children("conflict1", "conflict3")
	tf.Assert.Parents("conflict3", "conflict1", "conflict2")

	require.NoError(t, tf.CreateConflict("conflict2.5", []string{"conflict1", "conflict2"}, []string{"conflict2.5"}))
	require.NoError(t, tf.UpdateConflictParents("conflict3", "conflict2.5", "conflict1", "conflict2"))
	tf.Assert.Children("conflict1", "conflict2.5")
	tf.Assert.Children("conflict2", "conflict2.5")
	tf.Assert.Children("conflict2.5", "conflict3")
	tf.Assert.Parents("conflict3", "conflict2.5")
	tf.Assert.Parents("conflict2.5", "conflict1", "conflict2")
}

func CreateConflict[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))
	tf.Assert.ConflictSetMembers("resource1", "conflict1", "conflict2")

	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}))
	require.NoError(t, tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}))
	tf.Assert.ConflictSetMembers("resource2", "conflict3", "conflict4")
	tf.Assert.Children("conflict1", "conflict3", "conflict4")
	tf.Assert.Parents("conflict3", "conflict1")
	tf.Assert.Parents("conflict4", "conflict1")
}

func LikedInstead[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	tf.Validators.CreateID("zero-weight")

	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CastVotes("zero-weight", 1, "conflict1"))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))
	tf.Assert.ConflictSetMembers("resource1", "conflict1", "conflict2")
	tf.Assert.LikedInstead([]string{"conflict1", "conflict2"}, "conflict1")

	require.Error(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))
	require.Error(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))

	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}))
	require.NoError(t, tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}))
	require.NoError(t, tf.CastVotes("zero-weight", 1, "conflict4"))
	tf.Assert.LikedInstead([]string{"conflict1", "conflict2", "conflict3", "conflict4"}, "conflict1", "conflict4")
}

func ConflictAcceptance[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	tf.Validators.CreateID("nodeID1", 10)
	tf.Validators.CreateID("nodeID2", 10)
	tf.Validators.CreateID("nodeID3", 10)
	tf.Validators.CreateID("nodeID4", 10)

	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))
	tf.Assert.ConflictSetMembers("resource1", "conflict1", "conflict2")
	tf.Assert.ConflictSets("conflict1", "resource1")
	tf.Assert.ConflictSets("conflict2", "resource1")

	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}))
	require.NoError(t, tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}))
	tf.Assert.ConflictSetMembers("resource2", "conflict3", "conflict4")
	tf.Assert.Children("conflict1", "conflict3", "conflict4")
	tf.Assert.Parents("conflict3", "conflict1")
	tf.Assert.Parents("conflict4", "conflict1")

	require.NoError(t, tf.CastVotes("nodeID1", 1, "conflict4"))
	require.NoError(t, tf.CastVotes("nodeID2", 1, "conflict4"))
	require.NoError(t, tf.CastVotes("nodeID3", 1, "conflict4"))

	tf.Assert.LikedInstead([]string{"conflict1"})
	tf.Assert.LikedInstead([]string{"conflict2"}, "conflict1")
	tf.Assert.LikedInstead([]string{"conflict3"}, "conflict4")
	tf.Assert.LikedInstead([]string{"conflict4"})

	tf.Assert.Accepted("conflict1", "conflict4")
}

func CastVotes[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	tf.Validators.CreateID("nodeID1", 10)
	tf.Validators.CreateID("nodeID2", 10)
	tf.Validators.CreateID("nodeID3", 10)
	tf.Validators.CreateID("nodeID4", 10)

	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))
	tf.Assert.ConflictSetMembers("resource1", "conflict1", "conflict2")
	tf.Assert.ConflictSets("conflict1", "resource1")
	tf.Assert.ConflictSets("conflict2", "resource1")

	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}))
	require.NoError(t, tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}))
	tf.Assert.ConflictSetMembers("resource2", "conflict3", "conflict4")
	tf.Assert.Children("conflict1", "conflict3", "conflict4")
	tf.Assert.Parents("conflict3", "conflict1")
	tf.Assert.Parents("conflict4", "conflict1")

	require.NoError(t, tf.CastVotes("nodeID1", 1, "conflict2"))
	require.NoError(t, tf.CastVotes("nodeID2", 1, "conflict2"))
	require.NoError(t, tf.CastVotes("nodeID3", 1, "conflict2"))
	tf.Assert.LikedInstead([]string{"conflict1"}, "conflict2")
	tf.Assert.Rejected("conflict1")
	tf.Assert.Accepted("conflict2")
	tf.Assert.Rejected("conflict3")
	tf.Assert.Rejected("conflict4")

	require.Error(t, tf.CastVotes("nodeID3", 1, "conflict1", "conflict2"))
}

func CastVotesVotePower[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	tf.Validators.CreateID("nodeID1", 10)
	tf.Validators.CreateID("nodeID2", 10)
	tf.Validators.CreateID("nodeID3", 10)
	tf.Validators.CreateID("nodeID4", 0)

	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))
	tf.Assert.ConflictSetMembers("resource1", "conflict1", "conflict2")
	tf.Assert.ConflictSets("conflict1", "resource1")
	tf.Assert.ConflictSets("conflict2", "resource1")

	// create nested conflicts
	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}))
	require.NoError(t, tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}))
	tf.Assert.ConflictSetMembers("resource2", "conflict3", "conflict4")
	tf.Assert.Children("conflict1", "conflict3", "conflict4")
	tf.Assert.Parents("conflict3", "conflict1")
	tf.Assert.Parents("conflict4", "conflict1")

	// casting a vote from non-relevant validator before any relevant validators increases validator weight
	require.NoError(t, tf.CastVotes("nodeID4", 2, "conflict3"))
	tf.Assert.LikedInstead([]string{"conflict1"})
	tf.Assert.LikedInstead([]string{"conflict2"}, "conflict1")
	tf.Assert.LikedInstead([]string{"conflict3"})
	tf.Assert.LikedInstead([]string{"conflict4"}, "conflict3")

	// casting a vote from non-relevant validator before any relevant validators increases validator weight
	require.NoError(t, tf.CastVotes("nodeID4", 2, "conflict2"))
	require.NoError(t, tf.CastVotes("nodeID4", 2, "conflict2"))
	tf.Assert.LikedInstead([]string{"conflict1"}, "conflict2")
	tf.Assert.LikedInstead([]string{"conflict2"})
	tf.Assert.LikedInstead([]string{"conflict3"}, "conflict2")
	tf.Assert.LikedInstead([]string{"conflict4"}, "conflict2")

	// casting a vote from a validator updates the validator weight
	require.NoError(t, tf.CastVotes("nodeID1", 2, "conflict4"))
	tf.Assert.LikedInstead([]string{"conflict1"})
	tf.Assert.LikedInstead([]string{"conflict2"}, "conflict1")
	tf.Assert.LikedInstead([]string{"conflict3"}, "conflict4")
	tf.Assert.LikedInstead([]string{"conflict4"})

	// casting a vote from non-relevant validator after processing a vote from relevant validator doesn't change weights
	require.NoError(t, tf.CastVotes("nodeID4", 2, "conflict2"))
	require.NoError(t, tf.CastVotes("nodeID4", 2, "conflict2"))
	tf.Assert.LikedInstead([]string{"conflict1"})
	tf.Assert.LikedInstead([]string{"conflict2"}, "conflict1")
	tf.Assert.LikedInstead([]string{"conflict3"}, "conflict4")
	tf.Assert.LikedInstead([]string{"conflict4"})
	tf.Assert.ValidatorWeight("conflict1", 10)
	tf.Assert.ValidatorWeight("conflict2", 0)
	tf.Assert.ValidatorWeight("conflict3", 0)
	tf.Assert.ValidatorWeight("conflict4", 10)

	// casting vote with lower vote power doesn't change the weights of conflicts
	require.NoError(t, tf.CastVotes("nodeID1", 1), "conflict3")
	tf.Assert.LikedInstead([]string{"conflict1"})
	tf.Assert.LikedInstead([]string{"conflict2"}, "conflict1")
	tf.Assert.LikedInstead([]string{"conflict3"}, "conflict4")
	tf.Assert.LikedInstead([]string{"conflict4"})
	tf.Assert.ValidatorWeight("conflict1", 10)
	tf.Assert.ValidatorWeight("conflict2", 0)
	tf.Assert.ValidatorWeight("conflict3", 0)
	tf.Assert.ValidatorWeight("conflict4", 10)

	// casting vote with higher vote power changes the weights of conflicts
	require.NoError(t, tf.CastVotes("nodeID1", 3, "conflict3"))
	tf.Assert.LikedInstead([]string{"conflict1"})
	tf.Assert.LikedInstead([]string{"conflict2"}, "conflict1")
	tf.Assert.LikedInstead([]string{"conflict3"})
	tf.Assert.LikedInstead([]string{"conflict4"}, "conflict3")
	tf.Assert.ValidatorWeight("conflict1", 10)
	tf.Assert.ValidatorWeight("conflict2", 0)
	tf.Assert.ValidatorWeight("conflict3", 10)
	tf.Assert.ValidatorWeight("conflict4", 0)
}

func CastVotesAcceptance[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *Framework[ConflictID, ResourceID, VotePower]) {
	tf.Validators.CreateID("nodeID1", 10)
	tf.Validators.CreateID("nodeID2", 10)
	tf.Validators.CreateID("nodeID3", 10)
	tf.Validators.CreateID("nodeID4", 10)

	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource1"}))
	tf.Assert.ConflictSetMembers("resource1", "conflict1", "conflict2")
	tf.Assert.ConflictSets("conflict1", "resource1")
	tf.Assert.ConflictSets("conflict2", "resource1")

	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}))
	require.NoError(t, tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}))
	tf.Assert.ConflictSetMembers("resource2", "conflict3", "conflict4")
	tf.Assert.Children("conflict1", "conflict3", "conflict4")
	tf.Assert.Parents("conflict3", "conflict1")
	tf.Assert.Parents("conflict4", "conflict1")

	require.NoError(t, tf.CastVotes("nodeID1", 1, "conflict3"))
	require.NoError(t, tf.CastVotes("nodeID2", 1, "conflict3"))
	require.NoError(t, tf.CastVotes("nodeID3", 1, "conflict3"))
	tf.Assert.LikedInstead([]string{"conflict1"})
	tf.Assert.Accepted("conflict1")
	tf.Assert.Rejected("conflict2")
	tf.Assert.Accepted("conflict3")
	tf.Assert.Rejected("conflict4")
}
