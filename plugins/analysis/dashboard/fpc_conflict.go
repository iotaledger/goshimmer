package dashboard

// conflictSet is defined as a a map of conflict IDs and their conflict.
type conflictSet = map[string]conflict

// Conflict defines the struct for the opinions of the nodes regarding a given conflict.
type conflict struct {
	nodesView map[string]voteContext `json:"nodesview"`
}

type voteContext struct {
	nodeID   string  `json:"nodeid"`
	rounds   int     `json:"rounds"`
	opinions []int32 `json:"opinions"`
	status   int32   `json:"status"`
}

func newConflict() conflict {
	return conflict{
		nodesView: make(map[string]voteContext),
	}
}

// isFinalized return true if all the nodes have finalized a given conflict.
// It also returns false if the given conflict has an empty nodesView.
func (c conflict) isFinalized() bool {
	if len(c.nodesView) == 0 {
		return false
	}

	count := 0
	for _, context := range c.nodesView {
		if context.status == liked || context.status == disliked {
			count++
		}
	}

	return (count == len(c.nodesView))
}

// finalizationStatus returns the ratio of nodes that have finlized a given conflict.
func (c conflict) finalizationStatus() float64 {
	if len(c.nodesView) == 0 {
		return 0
	}
	count := 0
	for _, context := range c.nodesView {
		if context.status == liked || context.status == disliked {
			count++
		}
	}

	return (float64(count) / float64(len(c.nodesView)))
}
