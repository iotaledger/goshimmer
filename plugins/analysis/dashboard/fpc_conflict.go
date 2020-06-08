package dashboard

// conflictSet is defined as a a map of conflict IDs and their conflict.
type conflictSet = map[string]conflict

// conflict defines the struct for the opinions of the nodes regarding a given conflict.
type conflict struct {
	NodesView map[string]voteContext `json:"nodesview" bson:"nodesview"`
}

type voteContext struct {
	NodeID   string  `json:"nodeid" bson:"nodeid"`
	Rounds   int     `json:"rounds" bson:"rounds"`
	Opinions []int32 `json:"opinions" bson:"opinions"`
	Status   int32   `json:"status" bson:"status"`
}

func newConflict() conflict {
	return conflict{
		NodesView: make(map[string]voteContext),
	}
}

// isFinalized return true if all the nodes have finalized a given conflict.
// It also returns false if the given conflict has an empty nodesView.
func (c conflict) isFinalized() bool {
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
func (c conflict) finalizationStatus() float64 {
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
