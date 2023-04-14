package newconflictdag

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
)

func TestConflictDAG_UpdateConflictParents(t *testing.T) {
	tf := NewTestFramework(t)

	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err2)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1", "conflict2"}, []string{"resource1", "resource2"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err3)

	fmt.Println(conflict1.Children)
	require.Equal(t, 1, conflict1.Children.Size())
	require.True(t, conflict1.Children.Has(conflict3))

	require.Equal(t, 1, conflict2.Children.Size())
	require.True(t, conflict2.Children.Has(conflict3))

	require.Equal(t, 2, conflict3.Parents.Size())
	require.True(t, conflict3.Parents.Has(conflict1))
	require.True(t, conflict3.Parents.Has(conflict2))

	conflict25, err25 := tf.CreateConflict("conflict2.5", []string{"conflict1", "conflict2"}, []string{"conflict2.5"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err25)

	require.NoError(t, tf.UpdateConflictParents("conflict3", "conflict2.5", "conflict1", "conflict2"))

	require.Equal(t, 1, conflict1.Children.Size())
	require.True(t, conflict1.Children.Has(conflict25))

	require.Equal(t, 1, conflict2.Children.Size())
	require.True(t, conflict2.Children.Has(conflict25))

	require.Equal(t, 1, conflict3.Parents.Size())
	require.True(t, conflict3.Parents.Has(conflict25))

	require.Equal(t, 2, conflict25.Parents.Size())
	require.True(t, conflict25.Parents.Has(conflict1))
	require.True(t, conflict25.Parents.Has(conflict2))

	require.Equal(t, 1, conflict25.Children.Size())
	require.True(t, conflict25.Children.Has(conflict3))
}

func TestConflictDAG_JoinConflictSets(t *testing.T) {
	tf := NewTestFramework(t)

	_, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	conflict2.setAcceptanceState(acceptance.Rejected)

	// test to modify non-existing conflict
	require.ErrorIs(t, tf.ConflictDAG.JoinConflictSets(NewTestID("conflict3"), NewTestID("resource2")), ErrEntityEvicted)

	// test to modify conflict with non-existing resource
	require.ErrorIs(t, tf.ConflictDAG.JoinConflictSets(NewTestID("conflict2"), NewTestID("resource2")), ErrEntityEvicted)

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

	tf := NewTestFramework(t, WithWeights(weights))

	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err4)

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{
		VotePower: 10,
	}), "conflict2"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID2"], vote.MockedPower{
		VotePower: 10,
	}), "conflict2"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], vote.MockedPower{
		VotePower: 10,
	}), "conflict2"))

	require.Contains(t, tf.LikedInstead("conflict1"), conflict2)

	require.True(t, conflict1.IsRejected())
	require.True(t, conflict2.IsAccepted())
	require.True(t, conflict3.IsRejected())
	require.True(t, conflict4.IsRejected())

	require.Error(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], vote.MockedPower{
		VotePower: 10,
	}), "conflict1", "conflict2"))
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

	tf := NewTestFramework(t, WithWeights(weights))
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

	tf := NewTestFramework(t, WithWeights(weights))

	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err4)

	// casting a vote from non-relevant validator before any relevant validators increases cumulative weight
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID4"], vote.MockedPower{VotePower: 1}), "conflict3"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting a vote from a validator updates the validator weight
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 10}), "conflict4"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote from non-relevant validator after processing a vote from relevant validator doesn't increase cumulative weight
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID4"], vote.MockedPower{VotePower: 1}), "conflict3"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting vote with lower vote power doesn't change the weights of conflicts
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 5}), "conflict3"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power doesn't change weights
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 11}), "conflict4"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power on a different conflict changes the weights
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 12}), "conflict3"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()
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

	tf := NewTestFramework(t, WithWeights(weights))
	conflict1, err1 := tf.CreateConflict("conflict1", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(5))
	require.NoError(t, err1)

	conflict2, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	conflict3, err3 := tf.CreateConflict("conflict3", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(0))
	require.NoError(t, err3)

	conflict4, err4 := tf.CreateConflict("conflict4", []string{"conflict1"}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err4)

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{
		VotePower: 10,
	}), "conflict3"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID2"], vote.MockedPower{
		VotePower: 10,
	}), "conflict3"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], vote.MockedPower{
		VotePower: 10,
	}), "conflict3"))

	require.Equal(t, 0, len(tf.LikedInstead("conflict1")))

	require.True(t, conflict1.IsAccepted())
	require.True(t, conflict2.IsRejected())
	require.True(t, conflict3.IsAccepted())
	require.True(t, conflict4.IsRejected())
}

func TestConflictDAG_CreateConflict(t *testing.T) {
	tf := NewTestFramework(t)
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

	require.True(t, conflict1.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]]()))
	require.True(t, conflict2.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]]()))
	require.True(t, conflict3.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]](conflict1)))
	require.True(t, conflict4.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]](conflict1)))
}

func TestConflictDAG_LikedInstead(t *testing.T) {
	tf := NewTestFramework(t)

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

func requireConflicts(t *testing.T, conflicts []*Conflict[TestID, TestID, vote.MockedPower], expectedConflicts ...*Conflict[TestID, TestID, vote.MockedPower]) {
	require.Equalf(t, len(expectedConflicts), len(conflicts), "number of liked conflicts incorrect")

	for _, expectedConflict := range expectedConflicts {
		require.Contains(t, conflicts, expectedConflict, "conflict %s must be liked", expectedConflict.ID)
	}
}
