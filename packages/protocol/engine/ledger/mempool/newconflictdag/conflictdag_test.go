package newconflictdag

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
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

	conflictDAG := New[TestID, TestID, vote.MockedPower](weights.TotalWeight)
	conflicts := map[string]*conflict.Conflict[TestID, TestID, vote.MockedPower]{
		"1":   conflictDAG.CreateConflict(conflictIDs["1"], []TestID{}, []TestID{resourceIDs["1"]}, weight.New(weights).SetCumulativeWeight(5)),
		"2":   conflictDAG.CreateConflict(conflictIDs["2"], []TestID{}, []TestID{resourceIDs["2"]}, weight.New(weights).SetCumulativeWeight(5)),
		"3":   conflictDAG.CreateConflict(conflictIDs["3"], []TestID{conflictIDs["1"], conflictIDs["2"]}, []TestID{resourceIDs["3"]}, weight.New(weights).SetCumulativeWeight(5)),
		"2.5": conflictDAG.CreateConflict(conflictIDs["2.5"], []TestID{conflictIDs["1"], conflictIDs["2"]}, []TestID{resourceIDs["2.5"]}, weight.New(weights).SetCumulativeWeight(5)),
	}

	conflictDAG.UpdateConflictParents(conflictIDs["3"], conflictIDs["2.5"], conflictIDs["1"], conflictIDs["2"])

	fmt.Println(len(conflicts))
}

func TestConflictDAG_JoinConflictSets(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](weights.TotalWeight)
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))

	require.Nil(t, conflictDAG.JoinConflictSets(conflictID3, resourceID2))

	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{}, []TestID{resourceID2}, weight.New(weights).SetCumulativeWeight(1))

	conflictDAG.JoinConflictSets(conflictID1, resourceID2)

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

	conflictDAG := New[TestID, TestID, vote.MockedPower](weights.TotalWeight)
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

	require.True(t, conflict1.AcceptanceState().IsRejected())
	require.True(t, conflict2.AcceptanceState().IsAccepted())
	require.True(t, conflict3.AcceptanceState().IsRejected())
	require.True(t, conflict4.AcceptanceState().IsRejected())

	require.Error(t, conflictDAG.CastVotes(vote.NewVote(nodesByIdentity["nodeID3"], vote.MockedPower{
		VotePower: 10,
	}), conflictID1, conflictID2))
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

	conflictDAG := New[TestID, TestID, vote.MockedPower](weights.TotalWeight)
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

	require.True(t, conflict1.AcceptanceState().IsAccepted())
	require.True(t, conflict2.AcceptanceState().IsRejected())
	require.True(t, conflict3.AcceptanceState().IsAccepted())
	require.True(t, conflict4.AcceptanceState().IsRejected())
}

func TestConflictDAG_CreateConflict(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](weights.TotalWeight)
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

	require.True(t, conflict1.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID, vote.MockedPower]]()))
	require.True(t, conflict2.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID, vote.MockedPower]]()))
	require.True(t, conflict3.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID, vote.MockedPower]](conflict1)))
	require.True(t, conflict4.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID, vote.MockedPower]](conflict1)))
}

func TestConflictDAG_LikedInstead(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID, vote.MockedPower](weights.TotalWeight)
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(5))
	conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New(weights).SetCumulativeWeight(1))

	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, weight.New(weights))
	})
	require.Panics(t, func() {
		conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, weight.New(weights))
	})

	require.Equal(t, 1, len(conflictDAG.LikedInstead(conflictID1, conflictID2)))
	require.Contains(t, conflictDAG.LikedInstead(conflictID1, conflictID2), conflictID1)

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
	return strings.Replace(id.TransactionID.String(), "TransactionID(", "TestID(", 1)
}

func requireConflicts(t *testing.T, conflicts map[TestID]*conflict.Conflict[TestID, TestID, vote.MockedPower], expectedConflicts ...*conflict.Conflict[TestID, TestID, vote.MockedPower]) {
	require.Equal(t, len(expectedConflicts), len(conflicts))
	for _, expectedConflict := range expectedConflicts {
		require.Equal(t, conflicts[expectedConflict.ID], expectedConflict)
	}
}
