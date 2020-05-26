package dashboard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConflictRecordUpdate(t *testing.T) {
	// test ConflictRecord creation
	c := newConflictRecord(2)
	require.Equal(t, 2, int(c.size))

	// test first new update
	conflictA := conflict{
		NodesView: map[string]voteContext{
			"nodeA": {
				NodeID:   "nodeA",
				Rounds:   3,
				Opinions: []int32{disliked, liked, disliked},
				Status:   liked,
			},
		},
	}
	c.update("A", conflictA)

	require.Equal(t, conflictA, c.conflictSet["A"])
	require.Equal(t, 1, len(c.buffer))
	require.Contains(t, c.buffer, "A")

	// test second new update
	conflictB := conflict{
		NodesView: map[string]voteContext{
			"nodeB": {
				NodeID:   "nodeB",
				Rounds:   3,
				Opinions: []int32{disliked, liked, disliked},
				Status:   liked,
			},
		},
	}
	c.update("B", conflictB)

	require.Equal(t, conflictB, c.conflictSet["B"])
	require.Equal(t, 2, len(c.buffer))
	require.Contains(t, c.buffer, "B")

	// test modify existing entry
	conflictB = conflict{
		NodesView: map[string]voteContext{
			"nodeB": {
				NodeID:   "nodeB",
				Rounds:   4,
				Opinions: []int32{disliked, liked, disliked, liked},
				Status:   liked,
			},
		},
	}
	c.update("B", conflictB)

	require.Equal(t, conflictB, c.conflictSet["B"])
	require.Equal(t, 2, len(c.buffer))
	require.Contains(t, c.buffer, "B")

	// test last update and first update entry removal
	conflictC := conflict{
		NodesView: map[string]voteContext{
			"nodeC": {
				NodeID:   "nodeC",
				Rounds:   3,
				Opinions: []int32{disliked, liked, disliked},
				Status:   liked,
			},
		},
	}
	c.update("C", conflictC)

	require.Equal(t, conflictC, c.conflictSet["C"])
	require.Equal(t, 2, len(c.buffer))
	require.Contains(t, c.buffer, "C")

	require.NotContains(t, c.conflictSet, "A")
	require.NotContains(t, c.buffer, "A")

}
