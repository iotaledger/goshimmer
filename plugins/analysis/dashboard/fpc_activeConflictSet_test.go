package dashboard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActiveConflictsUpdate(t *testing.T) {
	// ConflictRecord creation
	c := newActiveConflictSet()

	// test first new update
	conflictA := conflict{
		NodesView: map[string]voteContext{
			"nodeA": {
				NodeID:   "nodeA",
				Rounds:   3,
				Opinions: []int32{disliked, liked, disliked},
				Outcome:  liked,
			},
		},
	}
	c.update("A", conflictA)

	require.Equal(t, conflictA.NodesView, c.conflictSet["A"].NodesView)

	// test second new update
	conflictB := conflict{
		NodesView: map[string]voteContext{
			"nodeB": {
				NodeID:   "nodeB",
				Rounds:   3,
				Opinions: []int32{disliked, liked, disliked},
				Outcome:  liked,
			},
		},
	}
	c.update("B", conflictB)

	require.Equal(t, conflictB.NodesView, c.conflictSet["B"].NodesView)

	// test modify existing entry
	conflictB = conflict{
		NodesView: map[string]voteContext{
			"nodeB": {
				NodeID:   "nodeB",
				Rounds:   4,
				Opinions: []int32{disliked, liked, disliked, liked},
				Outcome:  liked,
			},
		},
	}
	c.update("B", conflictB)
	require.Equal(t, conflictB.NodesView, c.conflictSet["B"].NodesView)

	// test  entry removal
	c.delete("B")
	require.NotContains(t, c.conflictSet, "B")
}
