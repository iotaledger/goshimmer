package dashboard

type ConflictSet = map[string]Conflict

// Conflict defines the struct for the opinions of the nodes regarding a given conflict.
type Conflict struct {
	NodesView map[string]voteContext `json:"nodesview"`
}

type voteContext struct {
	NodeID   string  `json:"nodeid"`
	Rounds   int     `json:"rounds"`
	Opinions []int32 `json:"opinions"`
	Status   int32   `json:"status"`
}

func newConflict() Conflict {
	return Conflict{
		NodesView: make(map[string]voteContext),
	}
}

// isFinalized return true if all the nodes have finalized a given conflict.
// It also returns false if the given conflict has an empty nodesView.
func (c Conflict) isFinalized() bool {
	if len(c.NodesView) == 0 {
		return false
	}

	count := 0
	for _, context := range c.NodesView {
		if context.Status == liked || context.Status == disliked {
			count++
		}
	}

	return (count == len(c.NodesView))
}

// finalizationStatus returns the ratio of nodes that have finlized a given conflict.
func (c Conflict) finalizationStatus() float64 {
	if len(c.NodesView) == 0 {
		return 0
	}
	count := 0
	for _, context := range c.NodesView {
		if context.Status == liked || context.Status == disliked {
			count++
		}
	}

	return (float64(count) / float64(len(c.NodesView)))
}
