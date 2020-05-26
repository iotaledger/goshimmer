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
		nodesView: map[string]voteContext{
			"nodeA": {
				nodeID:   "nodeA",
				rounds:   3,
				opinions: []int32{disliked, liked, disliked},
				status:   liked,
			},
		},
	}
	c.update("A", conflictA)

	require.Equal(t, conflictA, c.conflictSet["A"])
	require.Equal(t, 1, len(c.buffer))
	require.Contains(t, c.buffer, "A")

	// test second new update
	conflictB := conflict{
		nodesView: map[string]voteContext{
			"nodeB": {
				nodeID:   "nodeB",
				rounds:   3,
				opinions: []int32{disliked, liked, disliked},
				status:   liked,
			},
		},
	}
	c.update("B", conflictB)

	require.Equal(t, conflictB, c.conflictSet["B"])
	require.Equal(t, 2, len(c.buffer))
	require.Contains(t, c.buffer, "B")

	// test modify existing entry
	conflictB = conflict{
		nodesView: map[string]voteContext{
			"nodeB": {
				nodeID:   "nodeB",
				rounds:   4,
				opinions: []int32{disliked, liked, disliked, liked},
				status:   liked,
			},
		},
	}
	c.update("B", conflictB)

	require.Equal(t, conflictB, c.conflictSet["B"])
	require.Equal(t, 2, len(c.buffer))
	require.Contains(t, c.buffer, "B")

	// test last update and first update entry removal
	conflictC := conflict{
		nodesView: map[string]voteContext{
			"nodeC": {
				nodeID:   "nodeC",
				rounds:   3,
				opinions: []int32{disliked, liked, disliked},
				status:   liked,
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
