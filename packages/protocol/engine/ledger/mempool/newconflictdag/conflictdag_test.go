package newconflictdag

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
)

func TestConflictDAG_UpdateConflictParents(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictIDs := map[string]TestID{
		"1":   NewTestID("conflict1"),
		"2":   NewTestID("conflict2"),
		"2.5": NewTestID("conflict2.5"),
		"3":   NewTestID("conflict3"),
	}

	resourceIDs := map[string]TestID{
		"1":   NewTestID("resource1"),
		"2":   NewTestID("resource2"),
		"2.5": NewTestID("resource2.5"),
		"3":   NewTestID("resource3"),
	}

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflicts := map[string]*Conflict[TestID, TestID, vote.MockedPower]{
		"1": conflictDAG.CreateConflict(conflictIDs["1"], []TestID{}, []TestID{resourceIDs["1"]}, weight.New(weights).SetCumulativeWeight(5)),
		"2": conflictDAG.CreateConflict(conflictIDs["2"], []TestID{}, []TestID{resourceIDs["2"]}, weight.New(weights).SetCumulativeWeight(5)),
		"3": conflictDAG.CreateConflict(conflictIDs["3"], []TestID{conflictIDs["1"], conflictIDs["2"]}, []TestID{resourceIDs["3"]}, weight.New(weights).SetCumulativeWeight(5)),
	}

	fmt.Println(conflicts["1"].Children)
	require.Equal(t, 1, conflicts["1"].Children.Size())
	require.True(t, conflicts["1"].Children.Has(conflicts["3"]))

	require.Equal(t, 1, conflicts["2"].Children.Size())
	require.True(t, conflicts["2"].Children.Has(conflicts["3"]))

	require.Equal(t, 2, conflicts["3"].Parents.Size())
	require.True(t, conflicts["3"].Parents.Has(conflicts["1"]))
	require.True(t, conflicts["3"].Parents.Has(conflicts["2"]))

	conflicts["2.5"] = conflictDAG.CreateConflict(conflictIDs["2.5"], []TestID{conflictIDs["1"], conflictIDs["2"]}, []TestID{resourceIDs["2.5"]}, weight.New(weights).SetCumulativeWeight(5))

	conflictDAG.UpdateConflictParents(conflictIDs["3"], conflictIDs["2.5"], conflictIDs["1"], conflictIDs["2"])

	require.Equal(t, 1, conflicts["1"].Children.Size())
	require.True(t, conflicts["1"].Children.Has(conflicts["2.5"]))

	require.Equal(t, 1, conflicts["2"].Children.Size())
	require.True(t, conflicts["2"].Children.Has(conflicts["2.5"]))

	require.Equal(t, 1, conflicts["3"].Parents.Size())
	require.True(t, conflicts["3"].Parents.Has(conflicts["2.5"]))

	require.Equal(t, 2, conflicts["2.5"].Parents.Size())
	require.True(t, conflicts["2.5"].Parents.Has(conflicts["1"]))
	require.True(t, conflicts["2.5"].Parents.Has(conflicts["2"]))

	require.Equal(t, 1, conflicts["2.5"].Children.Size())
	require.True(t, conflicts["2.5"].Children.Has(conflicts["3"]))
}

func TestConflictDAG_JoinConflictSets(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))

	require.Nil(t, conflictDAG.JoinConflictSets(conflictID3, resourceID2))

	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(1))

	require.NotEmpty(t, conflictDAG.JoinConflictSets(conflictID1, resourceID2))

	require.Empty(t, conflictDAG.JoinConflictSets(conflictID1, resourceID2))

	likedInstead := conflictDAG.LikedInstead(conflict1.ID, conflict2.ID, conflict3.ID)
	require.Contains(t, likedInstead, conflict1.ID)
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

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))
	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(1))

	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{
		VotePower: 10,
	}), conflictID2))

	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID2"], vote.MockedPower{
		VotePower: 10,
	}), conflictID2))

	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], vote.MockedPower{
		VotePower: 10,
	}), conflictID2))

	require.Contains(t, conflictDAG.LikedInstead(conflictID1), conflictID2)

	require.True(t, conflict1.IsRejected())
	require.True(t, conflict2.IsAccepted())
	require.True(t, conflict3.IsRejected())
	require.True(t, conflict4.IsRejected())

	require.Error(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], vote.MockedPower{
		VotePower: 10,
	}), conflictID1, conflictID2))
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

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))
	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(0))

	acceptedConflictWeight := weight.New(weights)
	acceptedConflictWeight.Validators.Add(nodesByIdentity["nodeID1"])
	acceptedConflictWeight.Validators.Add(nodesByIdentity["nodeID2"])
	acceptedConflictWeight.Validators.Add(nodesByIdentity["nodeID3"])
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, acceptedConflictWeight)

	require.Empty(t, conflictDAG.LikedInstead(conflictID1))
	require.Contains(t, conflictDAG.LikedInstead(conflictID2), conflictID1)
	require.Contains(t, conflictDAG.LikedInstead(conflictID3), conflictID4)
	require.Empty(t, conflictDAG.LikedInstead(conflictID4))

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

	conflictID1 := NewTestID("conflict1")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(1))

	// casting a vote from non-relevant validator before any relevant validators increases cumulative weight
	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID4"], vote.MockedPower{VotePower: 1}), conflictID3))
	conflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting a vote from a validator updates the validator weight
	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 10}), conflictID4))
	conflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote from non-relevant validator after processing a vote from relevant validator doesn't increase cumulative weight
	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID4"], vote.MockedPower{VotePower: 1}), conflictID3))
	conflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 1, conflict3.Weight.Value().CumulativeWeight())
	require.EqualValues(t, 6, conflict1.Weight.Value().CumulativeWeight())

	// casting vote with lower vote power doesn't change the weights of conflicts
	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 5}), conflictID3))
	conflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power doesn't change weights
	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 11}), conflictID4))
	conflictDAG.pendingTasks.WaitIsZero()

	require.EqualValues(t, 10, conflict1.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 0, conflict3.Weight.Value().ValidatorsWeight())
	require.EqualValues(t, 10, conflict4.Weight.Value().ValidatorsWeight())

	// casting a vote with higher power on a different conflict changes the weights
	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{VotePower: 12}), conflictID3))
	conflictDAG.pendingTasks.WaitIsZero()
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

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))
	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(1))

	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID1"], vote.MockedPower{
		VotePower: 10,
	}), conflictID3))

	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID2"], vote.MockedPower{
		VotePower: 10,
	}), conflictID3))

	require.NoError(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], vote.MockedPower{
		VotePower: 10,
	}), conflictID3))

	require.Equal(t, 0, len(conflictDAG.LikedInstead(conflictID1)))

	require.True(t, conflict1.IsAccepted())
	require.True(t, conflict2.IsRejected())
	require.True(t, conflict3.IsAccepted())
	require.True(t, conflict4.IsRejected())
}

func TestConflictDAG_CreateConflict(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](acceptance.ThresholdProvider(weights.TotalWeight))
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))
	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(1))

	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, weight.New(weights))
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, weight.New(weights))
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict3"), []TestID{}, []TestID{}, weight.New(weights))
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict4"), []TestID{}, []TestID{}, weight.New(weights))
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
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))

	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, weight.New(weights))
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, weight.New(weights))
	})

	requireConflicts(t, conflictDAG.LikedInstead(conflictID1, conflictID2), conflict1)

	conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(1))

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
