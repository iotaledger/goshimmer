package dashboard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConflictRecordUpdate(t *testing.T) {
	// ConflictRecord creation
	c := newConflictRecord()

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

	// test  entry removal
	c.delete("B")
	require.NotContains(t, c.conflictSet, "B")
}
