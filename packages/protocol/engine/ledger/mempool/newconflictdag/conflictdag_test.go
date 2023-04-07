package newconflictdag

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

func TestConflictDAG_CreateConflict(t *testing.T) {
	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID]()
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(5))
	conflict2 := conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(1))
	conflict3 := conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New().SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New().SetCumulativeWeight(1))

	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, weight.New()) })
	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, weight.New()) })
	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict3"), []TestID{}, []TestID{}, weight.New()) })
	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict4"), []TestID{}, []TestID{}, weight.New()) })

	require.Contains(t, conflictDAG.Conflicts(conflictID1), conflict1.ID)
	require.Contains(t, conflictDAG.Conflicts(conflictID2), conflict2.ID)
	require.Contains(t, conflictDAG.Conflicts(conflictID3), conflict3.ID)
	require.Contains(t, conflictDAG.Conflicts(conflictID4), conflict4.ID)

	require.True(t, conflict1.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID]]()))
	require.True(t, conflict2.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID]]()))
	require.True(t, conflict3.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID]](conflict1)))
	require.True(t, conflict4.Parents.Equal(advancedset.New[*conflict.Conflict[TestID, TestID]](conflict1)))
}

func TestConflictDAG_LikedInstead(t *testing.T) {
	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID]()
	conflict1 := conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(5))
	conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(1))

	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, weight.New()) })
	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, weight.New()) })

	require.Equal(t, 1, len(conflictDAG.LikedInstead(conflictID1, conflictID2)))
	require.Contains(t, conflictDAG.LikedInstead(conflictID1, conflictID2), conflictID1)

	conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New().SetCumulativeWeight(0))
	conflict4 := conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New().SetCumulativeWeight(1))

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

func requireConflicts(t *testing.T, conflicts map[TestID]*conflict.Conflict[TestID, TestID], expectedConflicts ...*conflict.Conflict[TestID, TestID]) {
	require.Equal(t, len(expectedConflicts), len(conflicts))
	for _, expectedConflict := range expectedConflicts {
		require.Equal(t, conflicts[expectedConflict.ID], expectedConflict)
	}
}
