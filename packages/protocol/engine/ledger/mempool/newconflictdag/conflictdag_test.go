package newconflictdag

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
)

func TestConflictDAG(t *testing.T) {
	conflictID1 := NewTestID("conflict1")
	conflictID2 := NewTestID("conflict2")
	resourceID1 := NewTestID("conflictingOutput1")

	conflictDAG := New[TestID, TestID]()
	conflictDAG.CreateConflict(conflictID1, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(5))
	conflictDAG.CreateConflict(conflictID2, []TestID{}, []TestID{resourceID1}, weight.New().SetCumulativeWeight(1))

	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict1"), []TestID{}, []TestID{}, weight.New()) })
	require.Panics(t, func() { conflictDAG.CreateConflict(NewTestID("conflict2"), []TestID{}, []TestID{}, weight.New()) })

	likedInsteadConflicts := conflictDAG.LikedInstead(conflictID1, conflictID2)
	require.Equal(t, 1, likedInsteadConflicts.Size())
	require.True(t, likedInsteadConflicts.Has(conflictID1))
}
