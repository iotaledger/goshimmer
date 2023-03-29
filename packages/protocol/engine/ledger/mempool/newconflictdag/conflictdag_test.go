package newconflictdag

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

func TestConflictDAG(t *testing.T) {
	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	conflictID3 := NewTestID("conflict3")
	conflictID4 := NewTestID("conflict4")
	resourceID1 := NewTestID("resource1")
	resourceID2 := NewTestID("resource2")

	conflictDAG := New[TestID, TestID]()
	conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(5))
	conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(1))

	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, weight.New()) })
	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, weight.New()) })

	require.True(t, conflictDAG.LikedInstead(conflictID1, conflictID2).Equal(advancedset.New[TestID](conflictID1)))

	conflictDAG.CreateConflict(conflictID3, []TestID{conflictID1}, []TestID{resourceID2}, weight.New().SetCumulativeWeight(0))
	conflictDAG.CreateConflict(conflictID4, []TestID{conflictID1}, []TestID{resourceID2}, weight.New().SetCumulativeWeight(1))

	require.True(t, conflictDAG.LikedInstead(conflictID1, conflictID2, conflictID3, conflictID4).Equal(advancedset.New[TestID](conflictID1, conflictID4)))
}
