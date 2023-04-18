package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
)

func Run[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](test *testing.T, testFrameworkProvider func(t2 *testing.T) *TestFramework[ConflictID, ResourceID, VotePower]) {
	for testName, testCase := range map[string]func(*testing.T, *TestFramework[ConflictID, ResourceID, VotePower]){
		"JoinConflictSets":      JoinConflictSets[ConflictID, ResourceID, VotePower],
		"UpdateConflictParents": UpdateConflictParents[ConflictID, ResourceID, VotePower],
	} {
		test.Run(testName, func(t *testing.T) { testCase(t, testFrameworkProvider(t)) })
	}
}

func JoinConflictSets[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *TestFramework[ConflictID, ResourceID, VotePower]) {
	require.NoError(tf.test, tf.CreateConflict("A", nil, []string{"A"}, acceptance.Pending))
}

func UpdateConflictParents[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](t *testing.T, tf *TestFramework[ConflictID, ResourceID, VotePower]) {
	require.NoError(t, tf.CreateConflict("conflict1", []string{}, []string{"resource1"}))
	require.NoError(t, tf.CreateConflict("conflict2", []string{}, []string{"resource2"}))

	require.NoError(t, tf.CreateConflict("conflict3", []string{"conflict1", "conflict2"}, []string{"resource1", "resource2"}))
	tf.AssertChildren("conflict1", "conflict3")
	tf.AssertParents("conflict3", "conflict1", "conflict2")

	require.NoError(t, tf.CreateConflict("conflict2.5", []string{"conflict1", "conflict2"}, []string{"conflict2.5"}))
	require.NoError(t, tf.UpdateConflictParents("conflict3", "conflict2.5", "conflict1", "conflict2"))
	tf.AssertChildren("conflict1", "conflict2.5")
	tf.AssertChildren("conflict2", "conflict2.5")
	tf.AssertChildren("conflict2.5", "conflict3")
	tf.AssertParents("conflict3", "conflict2.5")
	tf.AssertParents("conflict2.5", "conflict1", "conflict2")
}

/*
func TestConflictDAG_JoinConflictSets(t *testing.T) {
	tf := tests.NewTestFramework(t)

	_, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	conflict2.setAcceptanceState(acceptance.Rejected)

	// test to modify non-existing conflict
	require.ErrorIs(t, tf.Instance.JoinConflictSets(conflictdag.NewTestID("conflict3"), conflictdag.NewTestIDs("resource2")), conflictdag.ErrEntityEvicted)

	// test to modify conflict with non-existing resource
	require.ErrorIs(t, tf.Instance.JoinConflictSets(conflictdag.NewTestID("conflict2"), conflictdag.NewTestIDs("resource2")), conflictdag.ErrEntityEvicted)

	_, err3 := tf.CreateConflict("conflict3", []string{}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err3)

	require.NoError(t, tf.JoinConflictSets("conflict1", "resource2"))
	require.NoError(t, tf.JoinConflictSets("conflict1", "resource2"))

	likedInstead := tf.LikedInstead("conflict1", "conflict2", "conflict3")
	require.Contains(t, likedInstead, tf.Conflict("conflict1"))
	require.Equal(t, 1, len(likedInstead))
}

func TestConflictDAG_CastVotes(t *testing.T) {
	nodesByIdentity := map[string]identity.ID{
		"nodeID1": identity.GenerateIdentity().ID(),
		"nodeID2": identity.GenerateIdentity().ID(),
		"nodeID3": identity.GenerateIdentity().ID(),
		"nodeID4": identity.GenerateIdentity().ID(),
	}

	identityWeights := map[string]int64{
		"nodeID1": 10,
		"nodeID2": 10,
		"nodeID3": 10,
		"nodeID4": 10,
	}

	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	for alias := range nodesByIdentity {
		weights.Update(nodesByIdentity[alias], &sybilprotection.Weight{
			Value: identityWeights[alias],
		})
	}

	tf := tests.NewTestFramework(t, conflictdag.WithWeights(weights))

	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err4)

	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID1"], vote2.MockedPower(10)), "conflict2"))

	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID2"], vote2.MockedPower(10)), "conflict2"))

	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID3"], vote2.MockedPower(10)), "conflict2"))

	require.Contains(t, tf.LikedInstead("conflict1"), conflict2)

	require.True(t, conflict1.IsRejected())
	require.True(t, conflict2.IsAccepted())
	require.True(t, conflict3.IsRejected())
	require.True(t, conflict4.IsRejected())

	require.Error(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID3"], vote2.MockedPower(10)), "conflict1", "conflict2"))
}

func TestConflictDAG_CreateAcceptedConflict(t *testing.T) {
	nodesByIdentity := map[string]identity.ID{
		"nodeID1": identity.GenerateIdentity().ID(),
		"nodeID2": identity.GenerateIdentity().ID(),
		"nodeID3": identity.GenerateIdentity().ID(),
		"nodeID4": identity.GenerateIdentity().ID(),
	}

	identityWeights := map[string]int64{
		"nodeID1": 10,
		"nodeID2": 10,
		"nodeID3": 10,
		"nodeID4": 10,
	}

	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	for alias := range nodesByIdentity {
		weights.Update(nodesByIdentity[alias], &sybilprotection.Weight{
			Value: identityWeights[alias],
		})
	}

	tf := tests.NewTestFramework(t, conflictdag.WithWeights(weights))
	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)
	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)
	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	acceptedConflictWeight := tf.Weight()
	acceptedConflictWeight.Validators.Add(nodesByIdentity["nodeID1"])
	acceptedConflictWeight.Validators.Add(nodesByIdentity["nodeID2"])
	acceptedConflictWeight.Validators.Add(nodesByIdentity["nodeID3"])

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, acceptedConflictWeight)
	require.NoError(t, err4)

	require.Empty(t, tf.LikedInstead("conflict1"))
	require.Contains(t, tf.LikedInstead("conflict2"), conflict1)
	require.Contains(t, tf.LikedInstead("conflict3"), conflict4)
	require.Empty(t, tf.LikedInstead("conflict4"))

	require.True(t, conflict1.IsAccepted())
	require.True(t, conflict2.IsRejected())
	require.True(t, conflict3.IsRejected())
	require.True(t, conflict4.IsAccepted())
}

func TestConflictDAG_CastVotes2(t *testing.T) {
	nodesByIdentity := map[string]identity.ID{
		"nodeID1": identity.GenerateIdentity().ID(),
		"nodeID2": identity.GenerateIdentity().ID(),
		"nodeID3": identity.GenerateIdentity().ID(),
		"nodeID4": identity.GenerateIdentity().ID(),
	}

	identityWeights := map[string]int64{
		"nodeID1": 10,
		"nodeID2": 10,
		"nodeID3": 10,
		"nodeID4": 0,
	}

	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	for alias := range nodesByIdentity {
		weights.Update(nodesByIdentity[alias], &sybilprotection.Weight{
			Value: identityWeights[alias],
		})
	}

	tf := tests.NewTestFramework(t, conflictdag.WithWeights(weights))

	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err4)

	// casting a vote from non-relevant validator before any relevant validators increases cumulative weight
	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID4"], vote2.MockedPower(1)), "conflict3"))
	tf.Instance.pendingTasks.WaitIsZero()

	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting a vote from a validator updates the validator weight
	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID1"], vote2.MockedPower(10)), "conflict4"))
	tf.Instance.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote from non-relevant validator after processing a vote from relevant validator doesn't increase cumulative weight
	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID4"], vote2.MockedPower(1)), "conflict3"))
	tf.Instance.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting vote with lower vote power doesn't change the weights of conflicts
	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID1"], vote2.MockedPower(5)), "conflict3"))
	tf.Instance.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power doesn't change weights
	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID1"], vote2.MockedPower(11)), "conflict4"))
	tf.Instance.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power on a different conflict changes the weights
	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID1"], vote2.MockedPower(12)), "conflict3"))
	tf.Instance.pendingTasks.WaitIsZero()
	require.True(t, conflict4.IsPending())
	require.True(t, conflict1.IsPending())
	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict4.Weight.Value().ValidatorsWeight())
}

func TestConflictDAG_CastVotes1(t *testing.T) {
	nodesByIdentity := map[string]identity.ID{
		"nodeID1": identity.GenerateIdentity().ID(),
		"nodeID2": identity.GenerateIdentity().ID(),
		"nodeID3": identity.GenerateIdentity().ID(),
		"nodeID4": identity.GenerateIdentity().ID(),
	}

	identityWeights := map[string]int64{
		"nodeID1": 10,
		"nodeID2": 10,
		"nodeID3": 10,
		"nodeID4": 10,
	}

	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	for alias := range nodesByIdentity {
		weights.Update(nodesByIdentity[alias], &sybilprotection.Weight{
			Value: identityWeights[alias],
		})
	}

	tf := tests.NewTestFramework(t, conflictdag.WithWeights(weights))
	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err4)

	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID1"], vote2.MockedPower(10)), "conflict3"))

	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID2"], vote2.MockedPower(10)), "conflict3"))

	require.NoError(t, tf.CastVotes(vote2.NewVote(nodesByIdentity["nodeID3"], vote2.MockedPower(10)), "conflict3"))

	require.Equal(t, 0, len(tf.LikedInstead("conflict1")))

	require.True(t, conflict1.IsAccepted())
	require.True(t, conflict2.IsRejected())
	require.True(t, conflict3.IsAccepted())
	require.True(t, conflict4.IsRejected())
}

func TestConflictDAG_CreateConflict(t *testing.T) {
	tf := tests.NewTestFramework(t)
	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err4)

	require.Errorf(t, lo.Return2(tf.CreateConflict("conflict1", []string{}, []string{}, tf.Weight())), "conflict with id conflict1 already exists")
	require.Errorf(t, lo.Return2(tf.CreateConflict("conflict2", []string{}, []string{}, tf.Weight())), "conflict with id conflict2 already exists")
	require.Errorf(t, lo.Return2(tf.CreateConflict("conflict3", []string{}, []string{}, tf.Weight())), "conflict with id conflict3 already exists")
	require.Errorf(t, lo.Return2(tf.CreateConflict("conflict4", []string{}, []string{}, tf.Weight())), "conflict with id conflict4 already exists")

	require.True(t, conflict1.Parents.Equal(advancedset.New[*Conflict[conflictdag.TestID, conflictdag.TestID, vote2.MockedPower]]()))
	require.True(t, conflict2.Parents.Equal(advancedset.New[*Conflict[conflictdag.TestID, conflictdag.TestID, vote2.MockedPower]]()))
	require.True(t, conflict3.Parents.Equal(advancedset.New[*Conflict[conflictdag.TestID, conflictdag.TestID, vote2.MockedPower]](conflict1)))
	require.True(t, conflict4.Parents.Equal(advancedset.New[*Conflict[conflictdag.TestID, conflictdag.TestID, vote2.MockedPower]](conflict1)))
}

func TestConflictDAG_LikedInstead(t *testing.T) {
	tf := tests.NewTestFramework(t)

	conflict1, err := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err)

	_, err = tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err)

	require.Error(t, lo.Return2(tf.CreateConflict("conflict2", []string{}, []string{}, tf.Weight())))
	require.Error(t, lo.Return2(tf.CreateConflict("conflict2", []string{}, []string{}, tf.Weight())))

	requireConflicts(t, tf.LikedInstead("conflict1", "conflict2"), conflict1)

	_, err = tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err)

	conflict4, err := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err)

	requireConflicts(t, tf.LikedInstead("conflict1", "conflict2", "conflict3", "conflict4"), conflict1, conflict4)
}

func requireConflicts(t *testing.T, conflicts []*Conflict[conflictdag.TestID, conflictdag.TestID, vote2.MockedPower], expectedConflicts ...*Conflict[conflictdag.TestID, conflictdag.TestID, vote2.MockedPower]) {
	require.Equalf(t, len(expectedConflicts), len(conflicts), "number of liked conflicts incorrect")

	for _, expectedConflict := range expectedConflicts {
		require.Contains(t, conflicts, expectedConflict, "conflict %s must be liked", expectedConflict.ID)
	}
}
*/
