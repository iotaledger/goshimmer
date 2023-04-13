package newconflictdag

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
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

	tf.UpdateConflictParents("conflict3", "conflict2.5", "conflict1", "conflict2")

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

	_, err2 := tf.CreateConflict("conflict2", []string{}, []string{"resource1"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err2)

	require.Empty(t, tf.JoinConflictSets("conflict3", "resource2"))

	_, err3 := tf.CreateConflict("conflict3", []string{}, []string{"resource2"}, tf.Weight().SetCumulativeWeight(1))
	require.NoError(t, err3)

	require.NotEmpty(t, tf.JoinConflictSets("conflict1", "resource2"))

	require.Empty(t, tf.JoinConflictSets("conflict1", "resource2"))

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

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], votes.MockedVotePower{
		VotePower: 10,
	}), "conflict2"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID2"], votes.MockedVotePower{
		VotePower: 10,
	}), "conflict2"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], votes.MockedVotePower{
		VotePower: 10,
	}), "conflict2"))

	require.Contains(t, tf.LikedInstead("conflict1"), conflict2)

	require.True(t, conflict1.IsRejected())
	require.True(t, conflict2.IsAccepted())
	require.True(t, conflict3.IsRejected())
	require.True(t, conflict4.IsRejected())

	require.Error(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], votes.MockedVotePower{
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
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID4"], votes.MockedVotePower{VotePower: 1}), "conflict3"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting a vote from a validator updates the validator weight
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], votes.MockedVotePower{VotePower: 10}), "conflict4"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote from non-relevant validator after processing a vote from relevant validator doesn't increase cumulative weight
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID4"], votes.MockedVotePower{VotePower: 1}), "conflict3"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting vote with lower vote power doesn't change the weights of conflicts
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], votes.MockedVotePower{VotePower: 5}), "conflict3"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power doesn't change weights
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], votes.MockedVotePower{VotePower: 11}), "conflict4"))
	tf.ConflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power on a different conflict changes the weights
	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], votes.MockedVotePower{VotePower: 12}), "conflict3"))
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

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], votes.MockedVotePower{
		VotePower: 10,
	}), "conflict3"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID2"], votes.MockedVotePower{
		VotePower: 10,
	}), "conflict3"))

	require.NoError(t, tf.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], votes.MockedVotePower{
		VotePower: 10,
	}), "conflict3"))

	require.Equal(t, 0, len(tf.LikedInstead("conflict1")))

	require.True(t, conflict1.IsAccepted())
	require.True(t, conflict2.IsRejected())
	require.True(t, conflict3.IsAccepted())
	require.True(t, conflict4.IsRejected())
}

/*
func TestConflictDAG_CreateConflict(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, tf.Weight().SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, tf.Weight().SetCumulativeWeight(1))
	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, tf.Weight().SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, tf.Weight().SetCumulativeWeight(1))

	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, tf.Weight())
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, tf.Weight())
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict3"), []TestID{}, []TestID{}, tf.Weight())
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict4"), []TestID{}, []TestID{}, tf.Weight())
	})

	require.Contains(t, conflictDAG.Conflicts(conflictID1), conflict1.ID)
	require.Contains(t, conflictDAG.Conflicts(conflictID2), conflict2.ID)
	require.Contains(t, conflictDAG.Conflicts(conflictID3), conflict3.ID)
	require.Contains(t, conflictDAG.Conflicts(conflictID4), conflict4.ID)

	require.True(t, conflict1.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]]()))
	require.True(t, conflict2.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]]()))
	require.True(t, conflict3.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]](conflict1)))
	require.True(t, conflict4.Parents.Equal(advancedset.New[*Conflict[TestID, TestID, vote.MockedPower]](conflict1)))
}

func TestConflictDAG_LikedInstead(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, tf.Weight().SetCumulativeWeight(5))
	conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, tf.Weight().SetCumulativeWeight(1))

	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, tf.Weight())
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, tf.Weight())
	})

	requireConflicts(t, conflictDAG.LikedInstead(conflictID1, conflictID2), conflict1)

	conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, tf.Weight().SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, tf.Weight().SetCumulativeWeight(1))

	requireConflicts(t, conflictDAG.LikedInstead(conflictID1, conflictID2, conflictID3, conflictID4), conflict1, conflict4)
}

type TestID struct {
	utxo.TransactionID
}

func NewTestID(alias string) TestID {
	hashedAlias := blake2b.Sum256([]byte(alias))

	testID := utxo.NewTransactionID(hashedAlias[:])
	testID.RegisterAlias(alias)

	return TestID{testID}
}

func (id TestID) String() string {
	return strings.Replace(id.TransactionID.String(), "TransactionID", "TestID", 1)
}

func requireConflicts(t *testing.T, conflicts map[TestID]*Conflict[TestID, TestID, vote.MockedPower], expectedConflicts ...*Conflict[TestID, TestID, vote.MockedPower]) {
	require.Equalf(t, len(expectedConflicts), len(conflicts), "number of liked conflicts incorrect")
	for _, expectedConflict := range expectedConflicts {
		conflict, exists := conflicts[expectedConflict.ID]
		require.True(t, exists, "conflict %s must be liked. Actual LikedInstead IDs: %s", expectedConflict.ID, lo.Keys(conflicts))
		require.Equalf(t, conflict, expectedConflict, "conflicts with ID not equal %s", expectedConflict.ID)
	}
}

/**/
